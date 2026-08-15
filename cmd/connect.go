package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	osexec "os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/rs/zerolog/log"

	"github.com/davegallant/vpngate/internal/tui"
	"github.com/davegallant/vpngate/pkg/daemon"
	"github.com/davegallant/vpngate/pkg/vpn"
	"github.com/spf13/cobra"
)

var (
	flagRandom         bool
	flagReconnect      bool
	flagProxy          string
	flagSocks5Proxy    string
	flagDaemon         bool
	flagDaemonRun      bool
	flagDaemonHostname string
	flagBest           bool
	flagTUI            bool
	flagHealthTimeout  time.Duration
	flagWatch          bool
	flagWatchInterval  time.Duration
	flagTunnelHealth   bool
)

func init() {
	connectCmd.Flags().BoolVarP(&flagRandom, "random", "r", false, "connect to a random server")
	connectCmd.Flags().BoolVarP(&flagReconnect, "reconnect", "t", false, "continually attempt to connect to the server")
	connectCmd.Flags().StringVarP(&flagProxy, "proxy", "p", "", "provide a http/https proxy server to make requests through (i.e. http://127.0.0.1:8080)")
	connectCmd.Flags().StringVarP(&flagSocks5Proxy, "socks5", "s", "", "provide a socks5 proxy server to make requests through (i.e. 127.0.0.1:1080)")
	connectCmd.Flags().StringVar(&flagCountry, "country", "", "filter by country name or country code (i.e. Japan or jp)")
	connectCmd.Flags().IntVar(&flagMaxPing, "max-ping", 0, "filter out servers with ping higher than this value")
	connectCmd.Flags().IntVar(&flagMinScore, "min-score", 0, "filter out servers with score lower than this value")
	connectCmd.Flags().StringVar(&flagProto, "proto", "", "filter by tunnel transport (tcp or udp)")
	connectCmd.Flags().BoolVar(&flagRefresh, "refresh", false, "refresh the vpn server list cache before connecting")
	connectCmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "do not read from or write to the vpn server list cache")
	connectCmd.Flags().BoolVarP(&flagDaemon, "daemon", "d", false, "run the connection in the background; see 'vpngate status' and 'vpngate disconnect'")
	connectCmd.Flags().BoolVar(&flagDaemonRun, "__daemon-run", false, "internal: run as the background daemon supervisor")
	connectCmd.Flags().StringVar(&flagDaemonHostname, "__daemon-hostname", "", "internal: hostname resolved by the foreground process")
	_ = connectCmd.Flags().MarkHidden("__daemon-run")
	_ = connectCmd.Flags().MarkHidden("__daemon-hostname")
	connectCmd.Flags().BoolVar(&flagHealthCheck, "health-check", true, "probe servers with a real OpenVPN connection before selecting")
	connectCmd.Flags().IntVar(&flagHealthConcurrency, "health-concurrency", 10, "number of parallel health probes")
	connectCmd.Flags().DurationVar(&flagHealthTimeout, "health-timeout", 5*time.Second, "per-server health probe timeout")
	connectCmd.Flags().BoolVar(&flagWatch, "watch", true, "keep re-verifying server health in the background")
	connectCmd.Flags().DurationVar(&flagWatchInterval, "watch-interval", 30*time.Second, "how often to re-verify servers in the background")
	connectCmd.Flags().BoolVar(&flagTunnelHealth, "tunnel-health-check", true, "verify the live tunnel keeps carrying traffic and reconnect when it stops (disable to never drop a connected tunnel)")
	connectCmd.Flags().BoolVar(&flagBest, "best", false, "automatically select the fastest working server without prompting")
	connectCmd.Flags().BoolVar(&flagTUI, "tui", true, "use the interactive server picker instead of the plain survey")
	rootCmd.AddCommand(connectCmd)
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a vpn server (survey selection appears if hostname is not provided)",
	Long:  `Connect to a vpn from a list of relay servers. Because openvpn creates a network interface, run the connect command with 'sudo' or a user with escalated privileges.`,
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDaemonRun {
			return runSupervisor()
		}

		if err := validateProtoFlag(); err != nil {
			return err
		}

		vpnServers, err := vpn.GetListWithOptions(flagProxy, flagSocks5Proxy, vpn.ListOptions{Refresh: flagRefresh, NoCache: flagNoCache})
		if err != nil {
			return err
		}

		vpnServers = filterServers(vpnServers)
		if len(*vpnServers) == 0 {
			return fmt.Errorf("no vpn servers matched the provided filters")
		}

		interactive := flagTUI && terminalInteractive()
		var serverSelected vpn.Server
		probeResults := make(map[string]vpn.ProbeResult)

		// The TUI probes servers live in the background, so the blocking
		// verification round is skipped for it.
		if flagHealthCheck && !interactive {
			probeResults = runProbe(cmd.Context(), *vpnServers, flagHealthConcurrency, flagHealthTimeout)
			working := workingServers(*vpnServers, probeResults)

			log.Info().Msgf("%d of %d servers verified working", len(working), len(*vpnServers))
			if len(working) == 0 {
				return fmt.Errorf("no working vpn servers found")
			}
			vpnServers = &working
		}

		switch {
		case flagBest:
			results := probeResults
			if len(results) == 0 {
				log.Info().Msgf("Probing %d servers to find the fastest working one (timeout: %s, concurrency: %d)...", len(*vpnServers), flagHealthTimeout, flagHealthConcurrency)
				results = runProbe(cmd.Context(), *vpnServers, flagHealthConcurrency, flagHealthTimeout)
			}
			working := workingServers(*vpnServers, results)
			if len(working) == 0 {
				return fmt.Errorf("no working vpn servers found")
			}
			serverSelected = working[0]

		case flagRandom && interactive:
			// select randomly from the full (or working) list inside the loop

		case len(args) > 0:
			serverMap := make(map[string]vpn.Server, len(*vpnServers))
			for _, s := range *vpnServers {
				serverMap[s.HostName] = s
			}
			selection := args[0]
			if server, exists := serverMap[selection]; exists {
				serverSelected = server
			} else if server, exists := serverMap[extractHostname(selection)]; exists {
				serverSelected = server
			} else {
				return fmt.Errorf("server %q was not found", selection)
			}

		case interactive:
			if !flagDaemon {
				return runTuiConnect(cmd.Context(), *vpnServers)
			}
			server, ok, err := runTuiPicker(cmd.Context(), *vpnServers)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			serverSelected = server

		default:
			serverSelection, serverMap := buildServerSelection(*vpnServers)
			selection := ""
			prompt := &survey.Select{
				Message: "Choose a server:",
				Options: serverSelection,
			}
			if err := survey.AskOne(prompt, &selection, survey.WithPageSize(10)); err != nil {
				return fmt.Errorf("unable to obtain hostname from survey: %w", err)
			}

			// Lookup server from selection using map for O(1) lookup.
			if server, exists := serverMap[selection]; exists {
				serverSelected = server
			} else if server, exists := serverMap[extractHostname(selection)]; exists {
				serverSelected = server
			} else {
				return fmt.Errorf("server %q was not found", selection)
			}
		}

		if flagDaemon {
			return startDaemon(serverSelected)
		}

		for {
			if flagRandom {
				// Select a random server
				serverSelected = (*vpnServers)[rand.Intn(len(*vpnServers))]
			}

			log.Info().Msgf("Connecting to %s (%s) in %s", serverSelected.HostName, serverSelected.IPAddr, serverSelected.CountryLong)
			err = connectWithRetry(cmd.Context(), orderedCandidates(*vpnServers, serverSelected, probeResults), nil, tunnelHealthFromFlag())

			if !flagReconnect {
				if err != nil {
					return fmt.Errorf("vpn connection failed: %w%s", err, privilegeHint(err))
				}
				return nil
			}
		}
	},
}

// connectStartupDeadline is how long a single relay attempt may take to
// reach "Initialization Sequence Completed" before it is abandoned in
// favor of the next candidate. Overridable in tests.
var connectStartupDeadline = 40 * time.Second

// maxConnectAttempts caps how many relays are tried per connect.
const maxConnectAttempts = 8

// connectWithRetry attempts to establish a tunnel against each candidate
// relay in order and stops at the first relay that actually initializes a
// tunnel. A relay that refuses the connection (AUTH_FAILED, which vpngate
// relays commonly send when they are at capacity) or that fails to
// initialize within connectStartupDeadline is abandoned and the next
// candidate is tried. Once a tunnel is up the function blocks until ctx is
// canceled (the user stops the connection) or the tunnel drops.
func connectWithRetry(ctx context.Context, candidates []vpn.Server, emit func(string), health *tunnelHealth) error {
	if len(candidates) == 0 {
		return errors.New("no servers to try")
	}

	attempts := min(len(candidates), maxConnectAttempts)
	var reasons []string

	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := candidates[i]

		if emit != nil {
			emit(fmt.Sprintf("Connecting to %s (%s) in %s", s.HostName, s.IPAddr, s.CountryLong))
		}
		log.Info().Msgf("Attempt %d/%d: connecting to %s (%s)", i+1, attempts, s.HostName, s.IPAddr)

		result := connectAttempt(ctx, s, emit, health)

		switch {
		case result.err == nil && ctx.Err() == nil:
			// Tunnel was up and openvpn exited on its own: that is a
			// dropped tunnel, report it.
			reasons = append(reasons, fmt.Sprintf("%s: tunnel dropped", s.HostName))
			if emit != nil && i < attempts-1 {
				emit("[vpngate] tunnel dropped; reconnecting…")
			}
		case result.err == nil:
			// User stopped the connection.
			return nil
		case result.authFailed:
			reasons = append(reasons, fmt.Sprintf("%s: relay refused the connection (likely full)", s.HostName))
			if emit != nil && i < attempts-1 {
				emit(fmt.Sprintf("Relay %s refused the connection (AUTH_FAILED, likely full), trying next relay…", s.HostName))
			}
		default:
			if result.initialized && emit != nil && i < attempts-1 {
				emit("[vpngate] tunnel dropped; reconnecting…")
			}
			reasons = append(reasons, fmt.Sprintf("%s: %v", s.HostName, result.err))
			if emit != nil && i < attempts-1 {
				emit(fmt.Sprintf("Relay %s failed (%v), trying next relay…", s.HostName, result.err))
			}
		}

		if ctx.Err() != nil {
			// The user stopped mid-chain or during the last attempt: that
			// is a stop, not an all-failed report.
			return ctx.Err()
		}
	}

	return fmt.Errorf("all %d attempted relays failed: %s", attempts, strings.Join(reasons, "; "))
}

// connectAttemptResult is the outcome of a single relay attempt.
type connectAttemptResult struct {
	err        error
	authFailed bool
	// initialized is true when the tunnel reached "Initialization Sequence
	// Completed" before failing (dropped, killed by the health check, or
	// openvpn crashed). Used to tell the user the tunnel was up and lost.
	initialized bool
}

// connectAttempt runs openvpn against one relay and reports how it went:
// nil error with authFailed=false means the tunnel came up and stayed up
// until ctx was canceled or openvpn exited on its own. authFailed is set
// when the relay sent AUTH_FAILED. Any other error means the relay never
// produced a working tunnel within the startup deadline or exited
// abnormally before doing so.
func connectAttempt(ctx context.Context, s vpn.Server, emit func(string), health *tunnelHealth) connectAttemptResult {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tw := &trackedWriter{emit: emit, initDone: make(chan struct{}), authDone: make(chan struct{})}

	runErrCh := make(chan error, 1)
	go func() { runErrCh <- connectServer(attemptCtx, s, tw) }()

	timer := time.NewTimer(connectStartupDeadline)
	defer timer.Stop()

	select {
	case <-tw.initDone:
		// Tunnel established; hand over to the verification check and
		// run until openvpn exits or the user stops it. The health
		// monitor watches for a silently dead tunnel and cancels the
		// attempt so the retry chain picks the next relay.
		if emit != nil {
			emit("Tunnel up; verifying connectivity through it…")
			emit(fmt.Sprintf("[vpngate] connected via %s", s.HostName))
		}
		go verifyTunnel(emit)
		go keepTunnelAlive(attemptCtx, cancel, emit, health)
		err := <-runErrCh
		if ctx.Err() != nil {
			// User stop: not an error.
			return connectAttemptResult{}
		}
		return connectAttemptResult{err: err, initialized: true}

	case <-tw.authDone:
		cancel()
		<-runErrCh
		return connectAttemptResult{err: errors.New("relay refused the connection"), authFailed: true}

	case <-timer.C:
		cancel()
		<-runErrCh
		return connectAttemptResult{err: fmt.Errorf("no tunnel within %s", connectStartupDeadline)}

	case <-ctx.Done():
		// User canceled while connecting. Wait for openvpn to be torn
		// down so its output can never race with the caller closing the
		// output stream.
		cancel()
		<-runErrCh
		return connectAttemptResult{err: ctx.Err()}

	case err := <-runErrCh:
		// openvpn exited before the startup deadline. Classify the exit
		// against the marker channels: connectServer only sends on
		// runErrCh after openvpn's output has been fully drained, so
		// initDone/authDone are in their final state here and the nested
		// select is race-free.
		select {
		case <-tw.authDone:
			return connectAttemptResult{err: errors.New("relay refused the connection"), authFailed: true}
		case <-tw.initDone:
			return connectAttemptResult{err: err, initialized: true}
		default:
		}
		if ctx.Err() != nil {
			return connectAttemptResult{err: ctx.Err()}
		}
		if err == nil {
			err = errors.New("openvpn exited before initializing the tunnel")
		}
		return connectAttemptResult{err: err}
	}
}

// trackedWriter tees openvpn output lines to emit while watching for the
// markers that decide whether a relay attempt is established or refused.
type trackedWriter struct {
	emit     func(string)
	initDone chan struct{}
	authDone chan struct{}
	onceInit sync.Once
	onceAuth sync.Once
}

// Write splits p on newlines and delivers each non-empty line to emit,
// closing the init/auth signal channels when their markers appear.
func (w *trackedWriter) Write(p []byte) (int, error) {
	for _, l := range strings.Split(string(p), "\n") {
		if l == "" {
			continue
		}
		if w.emit != nil {
			w.emit(l)
		}
		switch {
		case strings.Contains(l, "Initialization Sequence Completed"):
			w.onceInit.Do(func() { close(w.initDone) })
		case strings.Contains(l, "AUTH_FAILED"):
			w.onceAuth.Do(func() { close(w.authDone) })
		}
	}
	return len(p), nil
}

// tunnelHealthInterval is how often a live tunnel health check runs.
// Overridable in tests.
var tunnelHealthInterval = 10 * time.Second

// tunnelHealthGrace is how long after the tunnel comes up before the first
// health check runs. vpngate relays are community boxes with slow, bursty
// egress: checking immediately would judge the tunnel on its very first
// seconds and kill relays that are perfectly usable once settled.
// Overridable in tests.
var tunnelHealthGrace = 30 * time.Second

// tunnelHealthMaxFails is how many consecutive failed health checks are
// tolerated before the tunnel is declared dead and the attempt restarts.
var tunnelHealthMaxFails = 5

// tunnelHealthTimeout bounds one round of probes. Overridable in tests.
var tunnelHealthTimeout = 6 * time.Second

// tunnelHealthEndpoints is the set of HTTPS probes used to decide whether a
// live tunnel still egresses. Any HTTP response (any status code) proves the
// tunnel forwards traffic, so a relay with partial egress — one ISP's network
// reachable but not another's, or DNS broken through the tunnel — is judged
// alive as long as at least one endpoint answers. The mix deliberately spans
// DNS-resolved hosts and pure-IP endpoints so a broken DNS resolver inside
// the tunnel cannot alone kill the connection. Overridable in tests.
var tunnelHealthEndpoints = []string{
	"https://www.gstatic.com/generate_204",
	"https://www.google.com/generate_204",
	"https://1.1.1.1/cdn-cgi/trace",
	"https://8.8.8.8/",
}

// tunnelHealthCheck performs one liveness probe of the tunnel: it fires the
// tunnelHealthEndpoints in parallel and reports success as soon as any of
// them answers with an HTTP response. Overridable in tests.
var tunnelHealthCheck = realTunnelHealthCheck

// realTunnelHealthCheck is the production probe behind tunnelHealthCheck.
func realTunnelHealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), tunnelHealthTimeout)
	defer cancel()

	var wg sync.WaitGroup
	firstErr := make(chan error, len(tunnelHealthEndpoints))
	ok := make(chan struct{}, 1)

	for _, ep := range tunnelHealthEndpoints {
		ep := ep
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
			if err != nil {
				firstErr <- err
				return
			}
			resp, err := (&http.Client{}).Do(req)
			if resp != nil {
				_ = resp.Body.Close()
			}
			if err != nil {
				firstErr <- err
				return
			}
			select {
			case ok <- struct{}{}:
			default:
			}
		}()
	}
	wg.Wait()

	select {
	case <-ok:
		return nil
	default:
	}
	return errors.New("no HTTPS through the tunnel (no response from any endpoint)")
}

// tunnelHealth controls the live-tunnel watchdog from a single owner.
// pause, when non-nil, lets the TUI suspend checks at runtime (the p key);
// while paused the watchdog skips probes and never accumulates failures, so
// a connected tunnel is never dropped by it.
type tunnelHealth struct {
	pause *atomic.Bool
}

// paused reports whether the watchdog is currently suspended.
func (h *tunnelHealth) paused() bool {
	return h != nil && h.pause != nil && h.pause.Load()
}

// tunnelHealthFromFlag builds the watchdog policy for a connect invocation.
// A nil return disables the watchdog entirely (--tunnel-health-check=false).
func tunnelHealthFromFlag() *tunnelHealth {
	if !flagTunnelHealth {
		return nil
	}
	return &tunnelHealth{}
}

// keepTunnelAlive periodically verifies that traffic actually egresses
// through the tunnel. openvpn exiting is not the only way a tunnel dies:
// relays can drop it administratively while the process keeps running.
// After tunnelHealthMaxFails consecutive failures the attempt context is
// canceled, openvpn is torn down, and the retry chain moves to the next
// relay (emitting the "tunnel dropped; reconnecting" marker through
// connectAttempt's result path). Stops when attemptCtx is done. A nil
// health disables monitoring. Checks never run during tunnelHealthGrace,
// and are skipped (never counted) while the watchdog is paused.
func keepTunnelAlive(ctx context.Context, cancel context.CancelFunc, emit func(string), health *tunnelHealth) {
	if health == nil {
		return
	}

	// Grace period: hold off the first check so a relay that is still
	// settling its egress is not judged on its first seconds.
	select {
	case <-ctx.Done():
		return
	case <-time.After(tunnelHealthGrace):
	}

	t := time.NewTicker(tunnelHealthInterval)
	defer t.Stop()

	fails := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		if health.paused() {
			// Suspended by the user (TUI p key): a paused watchdog must
			// not accumulate failures, otherwise resuming would trigger
			// an immediate kill for noise collected while paused.
			fails = 0
			continue
		}

		if err := tunnelHealthCheck(); err == nil {
			fails = 0
			continue
		} else {
			fails++
			if emit != nil {
				emit(fmt.Sprintf("[vpngate] WARNING: tunnel health check failed (%d/%d): %v", fails, tunnelHealthMaxFails, err))
			}
			if fails >= tunnelHealthMaxFails {
				if emit != nil {
					emit("[vpngate] tunnel appears dead; reconnecting…")
				}
				cancel()
				return
			}
		}
	}
}

// ipwhoResult is the subset of ipwho.is's JSON answered to an IP query.
type ipwhoResult struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
}

// geoClient is shared by the tunnel checks (verifyTunnel and the health
// monitor both need a client; a slow response must never block the other).
var geoHTTPClient = &http.Client{Timeout: 12 * time.Second}

// verifyTunnel performs an end-to-end check once a tunnel is up: an HTTPS
// fetch that exercises DNS resolution and egress through the tunnel, the
// tunnel's IPv4 exit IP, the exit's real geolocation, and whether IPv6
// egress still bypasses the tunnel. Results are emitted as log lines so
// the user can see in the live output whether the connection actually
// carries traffic and where it really exits.
func verifyTunnel(emit func(string)) {
	if emit == nil {
		return
	}

	start := time.Now()
	if err := tunnelHealthCheck(); err != nil {
		emit(fmt.Sprintf("[vpngate] WARNING: tunnel check failed: no HTTPS through the tunnel (%v)", err))
		return
	}
	ms := time.Since(start).Milliseconds()
	emit(fmt.Sprintf("[vpngate] tunnel verified: HTTPS through the tunnel in %dms", ms))

	// api4.ipify.org answers only over IPv4, so the exit IP reported is
	// the IPv4 tunnel exit regardless of host IPv6 configuration.
	ipResp, err := geoHTTPClient.Get("https://api4.ipify.org")
	if err != nil || ipResp.StatusCode != http.StatusOK {
		return
	}
	ip, _ := io.ReadAll(io.LimitReader(ipResp.Body, 64))
	_ = ipResp.Body.Close()
	exitIP := strings.TrimSpace(string(ip))
	if exitIP == "" {
		return
	}
	emit(fmt.Sprintf("[vpngate] exit IP: %s", exitIP))

	geoResp, err := geoHTTPClient.Get("https://ipwho.is/" + exitIP)
	if err != nil {
		emit(fmt.Sprintf("[vpngate] WARNING: exit geolocation failed (%v)", err))
	} else if geoResp.StatusCode != http.StatusOK {
		emit(fmt.Sprintf("[vpngate] WARNING: exit geolocation returned HTTP %d", geoResp.StatusCode))
		_ = geoResp.Body.Close()
	} else {
		var g ipwhoResult
		if jsonErr := json.NewDecoder(io.LimitReader(geoResp.Body, 4096)).Decode(&g); jsonErr == nil && g.CountryCode != "" {
			payload := fmt.Sprintf("%s · %s %s", g.IP, strings.ToUpper(g.CountryCode), g.Country)
			if g.City != "" {
				payload += " " + g.City
			}
			emit("[vpngate] exit: " + payload)
		}
		_ = geoResp.Body.Close()
	}

	// IPv6 egress is not routed through the tunnel: if the host can still
	// reach the internet over IPv6, browsers will bypass the VPN entirely.
	// The client config black-holes IPv6 (route-ipv6 ::/0); this check
	// flags the leak when that black-hole did not take effect.
	v6Resp, err := geoHTTPClient.Get("https://api6.ipify.org")
	if err != nil || v6Resp.StatusCode != http.StatusOK {
		return
	}
	v6IP, _ := io.ReadAll(io.LimitReader(v6Resp.Body, 64))
	_ = v6Resp.Body.Close()
	if strings.TrimSpace(string(v6IP)) != "" {
		emit(fmt.Sprintf("[vpngate] WARNING: IPv6 egress active (%s) — traffic can bypass the tunnel; disable IPv6 for full protection", strings.TrimSpace(string(v6IP))))
	}
}

// orderedCandidates returns the servers in connection attempt order: the
// preferred server first (when non-zero), then relays that verified
// working (lowest probe latency first), then the remaining relays ordered
// by their vpngate score so the most promising relays are tried first.
func orderedCandidates(servers []vpn.Server, preferred vpn.Server, results map[string]vpn.ProbeResult) []vpn.Server {
	cands := make([]vpn.Server, 0, len(servers))
	if preferred.HostName != "" {
		cands = append(cands, preferred)
	}
	for _, s := range servers {
		if s.HostName == preferred.HostName {
			continue
		}
		if r, ok := results[s.HostName]; ok && r.Status == vpn.ProbeWorking {
			cands = append(cands, s)
		}
	}
	working := cands[1:]
	sort.SliceStable(working, func(i, j int) bool {
		ri, rj := results[working[i].HostName], results[working[j].HostName]
		return ri.LatencyMs < rj.LatencyMs
	})

	for _, s := range servers {
		if s.HostName == preferred.HostName {
			continue
		}
		if r, ok := results[s.HostName]; !ok || r.Status != vpn.ProbeWorking {
			cands = append(cands, s)
		}
	}
	rest := cands[1+len(working):]
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Score > rest[j].Score })

	return cands
}

// connectServer decodes the server's embedded OpenVPN config and runs
// openvpn against it until it exits, streaming each output line to out
// (when non-nil). Debug verbosity (--verb 5) can be enabled with
// VPNGATE_DEBUG.
func connectServer(ctx context.Context, s vpn.Server, out io.Writer) error {
	decodedConfig, err := base64.StdEncoding.DecodeString(s.OpenVpnConfigData)
	if err != nil {
		return err
	}

	tmpfile, err := os.CreateTemp("", "vpngate-openvpn-config-")
	if err != nil {
		return err
	}

	if _, err := tmpfile.Write(decodedConfig); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return err
	}

	// vpngate relays are IPv4-only: on a host that also routes IPv6,
	// browsers would bypass the tunnel over IPv6 and reveal the real
	// location. Ask openvpn to fold IPv6 into the tunnel when the host
	// has a default IPv6 route; on IPv6-less hosts (where no leak is
	// possible) the directive is skipped so openvpn does not warn about
	// an IPv6 route it cannot apply.
	if hostRoutesIPv6() {
		if _, err := tmpfile.WriteString("\n# vpngate: force all IPv6 into the tunnel (relays are IPv4-only)\nroute-ipv6 ::/0\n"); err != nil {
			_ = tmpfile.Close()
			_ = os.Remove(tmpfile.Name())
			return err
		}
	}

	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpfile.Name())
		return err
	}

	defer func() { _ = os.Remove(tmpfile.Name()) }()

	verb := 4
	if os.Getenv("VPNGATE_DEBUG") != "" {
		verb = 5
		log.Debug().Msgf("debug: connecting with verbosity %d to %s (%s)", verb, s.HostName, s.IPAddr)
	}
	return vpn.ConnectContextWithVerb(ctx, tmpfile.Name(), out, verb)
}

// hostRoutesIPv6 reports whether the host has a default IPv6 route, i.e.
// real IPv6 connectivity that could bypass the IPv4-only tunnel. It reads
// /proc/net/ipv6_route (Linux) and treats absence as "no IPv6" everywhere.
func hostRoutesIPv6() bool {
	raw, err := os.ReadFile("/proc/net/ipv6_route")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "00000000000000000000000000000000" && f[1] == "00" {
			return true
		}
	}
	return false
}

// runTuiConnect opens the picker and connects to the chosen server inside
// the TUI, streaming openvpn output until the user stops it or openvpn
// exits, then returns to the picker. It returns when the user quits.
func runTuiConnect(ctx context.Context, servers []vpn.Server) error {
	restore := quietLogs()
	defer restore()

	// The live-tunnel watchdog runs in the TUI like everywhere else, but
	// its pause flag is shared with the model so the p key can suspend it
	// at runtime.
	var healthPause *atomic.Bool
	var health *tunnelHealth
	if flagTunnelHealth {
		healthPause = &atomic.Bool{}
		health = &tunnelHealth{pause: healthPause}
	}

	connectFn := func(connCtx context.Context, s vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		if err := connectWithRetry(connCtx, orderedCandidates(servers, s, results), emit, health); err != nil {
			return fmt.Errorf("%w%s", err, privilegeHint(err))
		}
		return nil
	}

	_, _, err := tui.Run(ctx, tui.Options{
		Servers:     servers,
		Concurrency: flagHealthConcurrency,
		Timeout:     flagHealthTimeout,
		Interval:    flagWatchInterval,
		Mode:        tui.ModeSelect,
		Watch:       flagWatch,
		ConnectFn:   connectFn,
		HealthPause: healthPause,
	})
	return err
}

// startDaemon re-execs the current binary detached from the terminal so
// it can run connect in the background, then waits for it to report a
// successful connection. serverSelected is the zero value when --random
// was passed — the daemon resolves its own server in that case, possibly
// reselecting on every reconnect attempt.
func startDaemon(serverSelected vpn.Server) error {
	if state, err := daemon.Load(); err == nil {
		if daemon.IsAlive(state.PID) {
			return fmt.Errorf("already connected to %s (PID %d); run 'vpngate disconnect' first", state.HostName, state.PID)
		}
		_ = daemon.Remove()
	} else if !os.IsNotExist(err) {
		return err
	}

	selfPath, err := os.Executable()
	if err != nil {
		return err
	}

	childArgs := []string{"connect", "--__daemon-run"}
	if !flagRandom {
		childArgs = append(childArgs, "--__daemon-hostname", serverSelected.HostName)
	}
	childArgs = append(childArgs, forwardableConnectArgs()...)

	child := osexec.Command(selfPath, childArgs...)
	child.SysProcAttr = daemon.DetachAttr()

	if err := child.Start(); err != nil {
		return fmt.Errorf("starting background daemon: %w", err)
	}

	// The child is not released: waitForDaemonReady must learn when the
	// daemon dies before ever reporting a connection (e.g. openvpn cannot
	// create a tun device), so a goroutine reaps it and closes
	// daemonExited.
	daemonExited := make(chan struct{})
	go func() {
		_ = child.Wait()
		close(daemonExited)
	}()

	return waitForDaemonReady(daemonExited, connectStartupDeadline)
}

// forwardableConnectArgs reproduces the subset of connect's own flags
// that the re-exec'd daemon supervisor needs to repeat the same server
// selection and connection behavior.
func forwardableConnectArgs() []string {
	var args []string
	if flagReconnect {
		args = append(args, "--reconnect")
	}
	if flagRandom {
		args = append(args, "--random")
	}
	if flagProxy != "" {
		args = append(args, "--proxy", flagProxy)
	}
	if flagSocks5Proxy != "" {
		args = append(args, "--socks5", flagSocks5Proxy)
	}
	if flagCountry != "" {
		args = append(args, "--country", flagCountry)
	}
	if flagMaxPing != 0 {
		args = append(args, "--max-ping", strconv.Itoa(flagMaxPing))
	}
	if flagMinScore != 0 {
		args = append(args, "--min-score", strconv.Itoa(flagMinScore))
	}
	if flagProto != "" {
		args = append(args, "--proto", flagProto)
	}
	if flagRefresh {
		args = append(args, "--refresh")
	}
	if flagNoCache {
		args = append(args, "--no-cache")
	}
	if flagHealthCheck {
		args = append(args, "--health-check")
	}
	if flagHealthConcurrency != 10 {
		args = append(args, "--health-concurrency", strconv.Itoa(flagHealthConcurrency))
	}
	if flagHealthTimeout != 5*time.Second {
		args = append(args, "--health-timeout", flagHealthTimeout.String())
	}
	return args
}

// waitForDaemonReady waits for the daemon's state file to appear,
// signalling a successful first connection. If the daemon process exits
// before that (daemonExited closes) or the timeout elapses, the tail of
// the daemon log is surfaced so the underlying failure reason (e.g.
// openvpn cannot create a tun device) is visible instead of a bare
// timeout.
func waitForDaemonReady(daemonExited <-chan struct{}, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		state, err := daemon.Load()
		if err == nil {
			fmt.Printf("Connected in background to %s (PID %d)\n", state.HostName, state.PID)
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-daemonExited:
			tail := tailLog()
			msg := fmt.Sprintf("background connection failed; see %s", daemon.LogPath())
			if tail != "" {
				msg += "\n" + tail + privilegeHint(errors.New(tail))
			}
			return errors.New(msg)
		case <-time.After(200 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for background connection; see %s\n%s", daemon.LogPath(), tailLog())
		}
	}
}

// tailLog returns the last few lines of the daemon log for error
// messages, or an empty string if it can't be read.
func tailLog() string {
	data, err := os.ReadFile(daemon.LogPath())
	if err != nil {
		return ""
	}
	return strings.TrimRight(lastLines(data, 10), "\n")
}

func buildServerSelection(servers []vpn.Server) ([]string, map[string]vpn.Server) {
	serverSelection := make([]string, len(servers))
	serverMap := make(map[string]vpn.Server, len(servers)*2)
	for i, server := range servers {
		label := formatServerSelection(server)
		serverSelection[i] = label
		serverMap[label] = server
		serverMap[server.HostName] = server
	}

	return serverSelection, serverMap
}

func formatServerSelection(server vpn.Server) string {
	flag := countryFlag(server.CountryShort)
	var label string
	if flag == "" {
		label = fmt.Sprintf("%s (%s)", server.CountryLong, server.IPAddr)
	} else {
		label = fmt.Sprintf("%s %s (%s)", flag, server.CountryLong, server.IPAddr)
	}

	if server.LatencyMs > 0 {
		return fmt.Sprintf("%s - %dms", label, server.LatencyMs)
	}
	return label
}

// countryFlag converts an ISO 3166-1 alpha-2 country code (e.g. "jp") into
// its regional indicator flag emoji (e.g. "🇯🇵"). It returns "" for codes
// that aren't exactly two ASCII letters.
func countryFlag(countryShort string) string {
	letters := []rune(strings.ToUpper(countryShort))
	if len(letters) != 2 {
		return ""
	}

	var flag strings.Builder
	for _, r := range letters {
		if r < 'A' || r > 'Z' {
			return ""
		}
		flag.WriteRune(0x1F1E6 + (r - 'A'))
	}
	return flag.String()
}

// privilegeHint returns a hint about running with elevated privileges when
// the failure looks like openvpn could not create the tun interface, or an
// empty string otherwise.
func privilegeHint(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "tunsetiff"), strings.Contains(msg, "not permitted"), strings.Contains(msg, "permission denied"):
		return " (openvpn needs elevated privileges to create a tun interface; re-run with sudo, e.g. 'sudo vpngate connect')"
	case strings.Contains(msg, "cannot open tun/tap"), strings.Contains(msg, "no such device"):
		return " (openvpn could not create the tun interface: the 'tun' kernel module is probably not loaded; run 'sudo modprobe tun' to load it)"
	}
	return ""
}

// extractHostname extracts the hostname from a manually provided argument or legacy selection string.
func extractHostname(selection string) string {
	selection = strings.TrimSpace(selection)

	parts := strings.Split(selection, " | ")
	if len(parts) > 0 {
		selection = strings.TrimSpace(parts[0])
	}

	parts = strings.Split(selection, " (")
	if len(parts) > 0 {
		selection = strings.TrimSpace(parts[0])
	}

	parts = strings.Fields(selection)
	if len(parts) > 0 {
		return parts[0]
	}

	return selection
}
