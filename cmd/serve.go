package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/davegallant/vpngate/pkg/daemon"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// serveDefaultAddr is where the GUI backend listens by default. The HTTP
// API it serves is the single contract shared by the desktop GUI (which
// spawns this command as a sidecar and talks to 127.0.0.1) and the mobile
// GUI (same frontend, pointed at a remote daemon started with
// --addr 0.0.0.0:1865).
const serveDefaultAddr = "127.0.0.1:1865"

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the GUI backend: an HTTP API owning one tunnel connection",
	Long: `Run the GUI backend: an HTTP API owning one tunnel connection.

The desktop app spawns this command as a sidecar and talks to it over
http://127.0.0.1:1865. The mobile app uses the same API against a remote
daemon (start it with --addr 0.0.0.0:1865). The CLI keeps working while
it runs: status and disconnect also answer over the daemon control socket.`,
	Args: cobra.NoArgs,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().String("addr", serveDefaultAddr, "address to listen on")
	rootCmd.AddCommand(serveCmd)
}

// serveAPI is the state behind the GUI backend: at most one supervisor
// (one tunnel) at a time.
type serveAPI struct {
	// fetchServers returns the server list to filter; injectable for tests.
	fetchServers func(opts vpn.ListOptions) ([]vpn.Server, error)

	mu  sync.Mutex
	sup *supervisor
	// done is closed when the current supervisor's run loop exits.
	done      chan struct{}
	monitor   *vpn.Monitor
	lastError string
}

// connectRequest is the body of POST /api/connect. hostname and random
// are mutually exclusive selectors; the remaining fields filter the
// candidate list first. reconnect defaults to true when omitted.
type connectRequest struct {
	HostName   string `json:"hostname"`
	Random     bool   `json:"random"`
	Proto      string `json:"proto"`
	Protocol   string `json:"protocol"`
	Transport  string `json:"transport"`
	Country    string `json:"country"`
	Source     string `json:"source"`
	Reconnect  *bool  `json:"reconnect"`
	KillSwitch bool   `json:"kill_switch"`
}

// apiServer is the JSON view of a relay for the GUI; OpenVpnConfigData is
// a large base64 blob the frontend never needs.
type apiServer struct {
	HostName     string `json:"hostname"`
	CountryLong  string `json:"country_long"`
	CountryShort string `json:"country_short"`
	Score        int    `json:"score"`
	IPAddr       string `json:"ip"`
	Ping         string `json:"ping"`
	Proto        string `json:"proto"`
	Transport    string `json:"transport,omitempty"`
	Source       string `json:"source,omitempty"`
	Health       string `json:"health,omitempty"`
	LatencyMs    int    `json:"latency_ms,omitempty"`
}

// statusResponse is what GET /api/status returns: the daemon snapshot
// plus the tunnel implementation details the snapshot does not carry.
type statusResponse struct {
	daemon.Snapshot
	Protocol  string `json:"protocol,omitempty"`
	Transport string `json:"transport,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runServe(cmd *cobra.Command, _ []string) error {
	addr, err := cmd.Flags().GetString("addr")
	if err != nil {
		return err
	}

	api := &serveAPI{
		fetchServers: func(opts vpn.ListOptions) ([]vpn.Server, error) {
			servers, err := vpn.GetListWithOptions(flagProxy, flagSocks5Proxy, opts)
			if err != nil {
				return nil, err
			}
			return *servers, nil
		},
	}

	// The desktop app spawns serve as a sidecar and never talks to it
	// over a terminal: when the app exits, the sidecar's stdin pipe
	// closes. Treat EOF on a piped stdin as a shutdown request so the
	// tunnel stops cleanly instead of being orphaned. Non-pipe stdin
	// (e.g. `serve </dev/null` from a shell) is left alone.
	shutdown := make(chan struct{})
	go func() {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeNamedPipe != 0 {
			_, _ = io.Copy(io.Discard, os.Stdin)
			close(shutdown)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	errCh := make(chan error, 1)
	go func() { errCh <- http.ListenAndServe(addr, api.handler()) }()

	log.Info().Msgf("GUI backend listening on http://%s", addr)
	select {
	case err := <-errCh:
		return err
	case <-interrupt:
	case <-shutdown:
	}

	log.Info().Msg("shutting down")
	api.shutdown()
	return nil
}

// shutdown stops any running tunnel and waits for the supervisor to exit,
// then releases its resources. It is used on SIGINT/SIGTERM and when the
// desktop app closes the sidecar's stdin pipe.
func (a *serveAPI) shutdown() {
	a.mu.Lock()
	if a.monitor != nil {
		a.monitor.Stop()
	}
	sup, done := a.sup, a.done
	a.mu.Unlock()
	if sup == nil {
		return
	}

	sup.handleStop()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}

	a.mu.Lock()
	if a.sup == sup {
		a.sup, a.done = nil, nil
	}
	a.mu.Unlock()
	sup.shutdown()
}

// handler wires the API routes; kept as a method so tests exercise the
// exact production mux (CORS included).
func (a *serveAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/ip", a.handleIP)
	mux.HandleFunc("/api/servers", a.handleServers)
	mux.HandleFunc("/api/servers/health", a.handleServersHealth)
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/connect", a.handleConnect)
	mux.HandleFunc("/api/disconnect", a.handleDisconnect)
	mux.HandleFunc("/api/logs", a.handleLogs)
	return withCORS(mux)
}

// withCORS allows any origin so the same frontend can later talk to a
// remote daemon from a mobile device.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *serveAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *serveAPI) handleIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.ipify.org?format=json")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "unable to fetch public IP: "+err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (a *serveAPI) handleServersHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	a.mu.Lock()
	mon := a.monitor
	a.mu.Unlock()

	if mon == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	results := mon.Results()
	out := make(map[string]map[string]any, len(results))
	for host, res := range results {
		health := "unknown"
		switch res.Status {
		case vpn.ProbeWorking:
			health = "working"
		case vpn.ProbeChecking:
			health = "checking"
		case vpn.ProbeAuthFailed, vpn.ProbeUnreachable, vpn.ProbeTimeout, vpn.ProbeError:
			health = "failed"
		}
		out[host] = map[string]any{
			"status":     health,
			"latency_ms": res.LatencyMs,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *serveAPI) handleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	q := r.URL.Query()
	servers, err := a.fetchServers(vpn.ListOptions{Refresh: q.Get("refresh") == "1"})
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetching servers: "+err.Error())
		return
	}

	a.mu.Lock()
	if a.monitor == nil && len(servers) > 0 {
		a.monitor = vpn.NewMonitor(servers, vpn.MonitorOptions{
			Concurrency: 8,
			Interval:    30 * time.Second,
			Continuous:  true,
		})
		a.monitor.Start()
	}
	var probeResults map[string]vpn.ProbeResult
	if a.monitor != nil {
		probeResults = a.monitor.Results()
	}
	a.mu.Unlock()

	minScore, _ := strconv.Atoi(q.Get("min_score"))
	maxPing, _ := strconv.Atoi(q.Get("max_ping"))
	servers = filterAPIServers(servers, q.Get("country"), q.Get("proto"), q.Get("transport"), q.Get("source"), minScore, maxPing)

	view := make([]apiServer, 0, len(servers))
	for _, s := range servers {
		var res *vpn.ProbeResult
		if probeResults != nil {
			if r, ok := probeResults[s.HostName]; ok {
				res = &r
			}
		}
		view = append(view, apiServerView(s, res))
	}
	writeJSON(w, http.StatusOK, view)
}

func (a *serveAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	a.mu.Lock()
	sup := a.sup
	lastErr := a.lastError
	a.mu.Unlock()

	if sup != nil {
		snap, err := sup.handleStatus()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sup.mu.Lock()
		supErr := sup.lastError
		sup.mu.Unlock()
		if snap.State == "CONNECTED" {
			lastErr = ""
		} else if supErr != "" {
			lastErr = supErr
		}
		writeJSON(w, http.StatusOK, statusResponse{Snapshot: snap, Protocol: sup.protocol, Transport: sup.transport, Error: lastErr})
		return
	}

	state, err := daemon.Load()
	if err == nil {
		if state.PID == os.Getpid() {
			// Stale state from this serve process's previous supervisor
			_ = daemon.Remove()
		} else if daemon.IsAlive(state.PID) {
			snap, err := daemon.SendStatus(state.ControlAddr, 500*time.Millisecond)
			if err == nil {
				writeJSON(w, http.StatusOK, statusResponse{Snapshot: snap, Error: lastErr})
				return
			}
			_ = daemon.Remove()
		} else {
			_ = daemon.Remove()
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		Snapshot: daemon.Snapshot{State: "DISCONNECTED"},
		Error:    lastErr,
	})
}

func (a *serveAPI) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	a.mu.Lock()
	busy := a.sup != nil
	a.mu.Unlock()
	if !busy {
		if state, err := daemon.Load(); err == nil {
			if state.PID == os.Getpid() {
				_ = daemon.Remove()
			} else if daemon.IsAlive(state.PID) {
				if _, err := daemon.SendStatus(state.ControlAddr, 500*time.Millisecond); err == nil {
					busy = true
				} else {
					_ = daemon.Remove()
				}
			} else {
				_ = daemon.Remove()
			}
		}
	}
	if busy {
		writeError(w, http.StatusConflict, "a connection is already running; disconnect first")
		return
	}

	servers, err := a.fetchServers(vpn.ListOptions{})
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetching servers: "+err.Error())
		return
	}

	proto := strings.ToLower(req.Proto)
	protocol := strings.ToLower(req.Protocol)
	transport := strings.ToLower(req.Transport)
	source := strings.ToLower(req.Source)

	// If protocol was sent as "udp" or "tcp", normalize to proto filter + openvpn protocol
	if protocol == "udp" || protocol == "tcp" {
		if proto == "" {
			proto = protocol
		}
		protocol = vpn.ProtocolOpenVPN
	}
	if protocol == "" {
		if source == vpn.SourceWarp {
			protocol = vpn.ProtocolWireGuard
		} else {
			protocol = vpn.ProtocolOpenVPN
		}
	}

	var initial vpn.Server
	var filtered []vpn.Server

	if req.HostName != "" {
		for _, s := range servers {
			if s.HostName == req.HostName {
				initial = s
				filtered = []vpn.Server{s}
				break
			}
		}
		if initial.HostName == "" {
			writeError(w, http.StatusNotFound, fmt.Sprintf("server %q was not found", req.HostName))
			return
		}
	} else {
		filtered = filterAPIServers(servers, req.Country, proto, transport, req.Source, 0, 0)
		if len(filtered) == 0 {
			writeError(w, http.StatusNotFound, "no servers matched the filters")
			return
		}
		if req.Random {
			initial = filtered[rand.Intn(len(filtered))]
		} else {
			initial = filtered[0]
		}
	}

	reconnect := true
	if req.Reconnect != nil {
		reconnect = *req.Reconnect
	}
	sup, err := newServeSupervisor(filtered, initial, protocol, transport, req.Random, reconnect, req.KillSwitch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	done := make(chan struct{})
	a.mu.Lock()
	if a.sup != nil {
		// A concurrent connect won the race after the early check.
		a.mu.Unlock()
		sup.shutdown()
		writeError(w, http.StatusConflict, "a connection is already running; disconnect first")
		return
	}
	a.lastError = ""
	a.sup, a.done = sup, done
	if a.monitor != nil {
		a.monitor.Pause()
	}
	a.mu.Unlock()

	go func() {
		defer func() {
			close(done)
			a.mu.Lock()
			if a.sup == sup {
				a.sup, a.done = nil, nil
			}
			if a.monitor != nil {
				a.monitor.Resume()
			}
			a.mu.Unlock()
			sup.shutdown()
		}()
		runErr := sup.run()
		if runErr != nil {
			a.mu.Lock()
			a.lastError = runErr.Error()
			a.mu.Unlock()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"state": "CONNECTING"})
}

func (a *serveAPI) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	a.mu.Lock()
	sup, done := a.sup, a.done
	if a.monitor != nil {
		a.monitor.Resume()
	}
	a.mu.Unlock()

	if sup != nil {
		sup.handleStop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			writeError(w, http.StatusGatewayTimeout, "tunnel did not stop in time")
			return
		}

		a.mu.Lock()
		if a.sup == sup {
			a.sup, a.done = nil, nil
		}
		a.lastError = ""
		a.mu.Unlock()
		_ = daemon.Remove()
		writeJSON(w, http.StatusOK, map[string]string{"state": "DISCONNECTED"})
		return
	}

	state, err := daemon.Load()
	if err == nil {
		if daemon.IsAlive(state.PID) && state.PID != os.Getpid() {
			if err := daemon.SendStop(state.ControlAddr, 5*time.Second); err != nil {
				if proc, ferr := os.FindProcess(state.PID); ferr == nil {
					_ = proc.Kill()
				}
			}
		}
		_ = daemon.Remove()
		writeJSON(w, http.StatusOK, map[string]string{"state": "DISCONNECTED"})
		return
	}

	writeError(w, http.StatusNotFound, "no connection to stop")
}

func (a *serveAPI) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 {
		n = 200
	}
	var buf bytes.Buffer
	if err := runLogs(&buf, daemon.LogPath(), n, false); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": buf.String()})
}

// newServeSupervisor builds a supervisor for the GUI backend: it opens
// the daemon log file and a loopback control listener (so `vpngate
// status`/`disconnect` keep working while the GUI is connected) and wires
// the same status/stop handlers the daemon re-exec path uses.
func newServeSupervisor(servers []vpn.Server, initial vpn.Server, protocol, transport string, random, reconnect, killSwitch bool) (*supervisor, error) {
	if err := os.MkdirAll(daemon.Dir(), 0o755); err != nil {
		return nil, err
	}
	_ = os.Chmod(daemon.Dir(), 0o755)
	logFile, err := os.OpenFile(daemon.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(daemon.LogPath(), 0o644)
	controlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = logFile.Close()
		return nil, err
	}

	s := &supervisor{
		vpnServers: servers,
		random:     random,
		reconnect:  reconnect,
		protocol:   protocol,
		transport:  transport,
		killSwitch: killSwitch,
		logFile:    logFile,
		server:     initial,
	}
	s.control = daemon.NewControlServer(controlLn, s.handleStatus, s.handleStop)
	go s.control.Serve()
	return s, nil
}

// shutdown releases the supervisor's control listener and log file
// without touching any tunnel. It is only used to clean up a supervisor
// that lost a connect race and never started.
func (s *supervisor) shutdown() {
	if s.ks != nil {
		_ = s.ks.Disable(context.Background())
	}
	if s.control != nil {
		_ = s.control.Close()
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
}

// filterAPIServers applies the GUI's filter parameters to servers, mirroring
// the CLI flag filters (country, proto, source, score/ping) plus the
// vpnbook transport filter. Unlike the CLI helpers it takes its inputs as
// parameters so the API never touches package-global flags.
func filterAPIServers(servers []vpn.Server, country, proto, transport, source string, minScore, maxPing int) []vpn.Server {
	country = strings.ToLower(country)
	proto = strings.ToLower(proto)
	transport = strings.ToLower(transport)

	filtered := make([]vpn.Server, 0, len(servers))
	for _, s := range servers {
		if country != "" && strings.ToLower(s.CountryShort) != country && !strings.Contains(strings.ToLower(s.CountryLong), country) {
			continue
		}
		if minScore > 0 && s.Score < minScore {
			continue
		}
		if maxPing > 0 {
			ping, err := strconv.Atoi(s.Ping)
			if err != nil || ping > maxPing {
				continue
			}
		}
		if proto != "" && s.Proto() != proto {
			continue
		}
		if source != "" && !strings.EqualFold(s.Source, source) {
			continue
		}
		if transport != "" && (s.Source != vpn.SourceVpnbook || !strings.EqualFold(s.Transport, transport)) {
			continue
		}
		filtered = append(filtered, s)
	}

	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Score > filtered[j].Score })
	return filtered
}

func apiServerView(s vpn.Server, res *vpn.ProbeResult) apiServer {
	view := apiServer{
		HostName:     s.HostName,
		CountryLong:  s.CountryLong,
		CountryShort: s.CountryShort,
		Score:        s.Score,
		IPAddr:       s.IPAddr,
		Ping:         s.Ping,
		Proto:        s.Proto(),
		Transport:    s.Transport,
		Source:       s.Source,
		Health:       "unknown",
	}
	if res != nil {
		switch res.Status {
		case vpn.ProbeWorking:
			view.Health = "working"
			view.LatencyMs = res.LatencyMs
		case vpn.ProbeChecking:
			view.Health = "checking"
		case vpn.ProbeAuthFailed, vpn.ProbeUnreachable, vpn.ProbeTimeout, vpn.ProbeError:
			view.Health = "failed"
		default:
			view.Health = "unknown"
		}
	}
	return view
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
