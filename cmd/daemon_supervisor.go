package cmd

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/davegallant/vpngate/pkg/daemon"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// supervisor owns a single daemon's lifecycle: it launches openvpn,
// tracks the currently-active management connection, answers control
// requests from separate `status`/`disconnect` invocations, and — when
// --reconnect is set — restarts openvpn if it exits on its own.
type supervisor struct {
	vpnServers []vpn.Server
	random     bool
	reconnect  bool
	// protocol and transport select the tunnel implementation and (for
	// vpnbook servers) the OpenVPN transport. They come from flags on the
	// re-exec'd daemon path and from the HTTP API on the serve path, so
	// they live on the supervisor rather than in package globals.
	protocol  string
	transport string
	logFile   *os.File
	control   *daemon.ControlServer

	mu        sync.Mutex
	server    vpn.Server
	startedAt time.Time
	mgmt      *daemon.Management
	stopping  bool
	// stateMu serializes management "state" queries. The management socket
	// is request/response and its bufio.Reader is not safe for concurrent
	// reads, and both waitForConnected (during a connect that can now take
	// up to connectStartupDeadline) and handleStatus (on a `status`
	// request) query it.
	stateMu sync.Mutex
}

// runSupervisor is the entry point used when connect is re-exec'd with
// --__daemon-run: it resolves the server to connect to, opens the
// control socket, and runs the connect/reconnect loop until told to
// stop.
func runSupervisor() error {
	vpnServers, err := vpn.GetListWithOptions(flagProxy, flagSocks5Proxy, vpn.ListOptions{Refresh: flagRefresh, NoCache: flagNoCache})
	if err != nil {
		return err
	}
	filtered := *filterServers(vpnServers)
	filtered = *filterTransportServers(&filtered, flagTransport)
	if len(filtered) == 0 {
		if flagTransport != "" {
			return fmt.Errorf("no vpnbook servers matched the provided filters (--transport is only supported for vpnbook servers)")
		}
		return fmt.Errorf("no vpn servers matched the provided filters")
	}

	var initial vpn.Server
	if flagRandom {
		initial = filtered[rand.Intn(len(filtered))]
	} else {
		found := false
		for _, s := range filtered {
			if s.HostName == flagDaemonHostname {
				initial = s
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("server %q was not found", flagDaemonHostname)
		}
	}

	if err := os.MkdirAll(daemon.Dir(), 0o755); err != nil {
		return err
	}
	_ = os.Chmod(daemon.Dir(), 0o755)
	logFile, err := os.OpenFile(daemon.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_ = os.Chmod(daemon.LogPath(), 0o644)
	defer func() { _ = logFile.Close() }()

	// The supervisor is a detached, re-exec'd child: nothing connects its
	// stdout/stderr back to the terminal the user is watching, so the
	// default console logger would silently discard everything (e.g. an
	// "openvpn is required" failure before openvpn ever starts, which
	// never reaches daemon.log otherwise). Redirect it to daemon.log,
	// the one place the foreground process points users at on failure.
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logFile, NoColor: true})

	controlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	s := &supervisor{
		vpnServers: filtered,
		random:     flagRandom,
		reconnect:  flagReconnect,
		protocol:   flagProtocol,
		transport:  flagTransport,
		logFile:    logFile,
		server:     initial,
	}
	s.control = daemon.NewControlServer(controlLn, s.handleStatus, s.handleStop)
	go s.control.Serve()

	return s.run()
}

func (s *supervisor) run() error {
	defer func() {
		_ = daemon.Remove()
		_ = os.Remove(daemon.ConfigPath())
	}()

	for {
		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			return nil
		}
		if s.random {
			s.server = s.vpnServers[rand.Intn(len(s.vpnServers))]
		}
		server := s.server
		s.mu.Unlock()

		err := s.connectOnce(server)
		if err != nil {
			log.Error().Err(err).Msg("daemon connection attempt failed")
			if !s.reconnect {
				return err
			}
			// Back off before retrying: a fast-failing attempt (e.g. an
			// instantly rejected server) must not spin this loop at 100% CPU.
			for i := 0; i < 10; i++ {
				s.mu.Lock()
				stopping := s.stopping
				s.mu.Unlock()
				if stopping {
					return nil
				}
				time.Sleep(200 * time.Millisecond)
			}
		}

		s.mu.Lock()
		stopping := s.stopping
		s.mu.Unlock()
		if stopping || !s.reconnect {
			return nil
		}
	}
}

// connectOnce starts openvpn for server, waits for it to report a
// successful connection, records daemon state, and blocks until it
// exits (either on its own or because handleStop signaled it).
func (s *supervisor) connectOnce(server vpn.Server) error {
	if server.Source == vpn.SourceWarp {
		return errors.New("WARP does not support background (daemon) mode")
	}
	if s.protocol != "" && s.protocol != vpn.ProtocolOpenVPN {
		return fmt.Errorf("background mode only supports the openvpn protocol (got %q)", s.protocol)
	}
	server, err := withRequestedTransport(server, s.transport)
	if err != nil {
		return err
	}
	configData, err := vpn.ServerConfig(server)
	if err != nil {
		return err
	}
	if err := os.WriteFile(daemon.ConfigPath(), configData, 0o600); err != nil {
		return err
	}

	mgmtAddr, err := reserveLoopbackAddr()
	if err != nil {
		return err
	}

	cmd, err := vpn.ClientFor(server).ConnectDetached(server, daemon.ConfigPath(), mgmtAddr, s.logFile, daemon.DetachAttr())
	if err != nil {
		return fmt.Errorf("starting openvpn: %w", err)
	}

	mgmt, err := s.waitForManagement(mgmtAddr, 30*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if errors.Is(err, errTunnelStopped) {
			return nil
		}
		return err
	}

	startedAt := time.Now()
	s.mu.Lock()
	if s.stopping {
		// disconnect() arrived while we were still connecting — undo
		// this attempt instead of publishing state for a connection
		// nobody asked for.
		s.mu.Unlock()
		_ = mgmt.Disconnect()
		_ = mgmt.Close()
		_ = cmd.Wait()
		return nil
	}
	s.server = server
	s.startedAt = startedAt
	s.mgmt = mgmt
	s.mu.Unlock()

	// The management interface comes up while openvpn is still handshaking
	// — and, in the worst case, about to die (e.g. "Cannot open TUN/TAP dev
	// /dev/net/tun"). Only publish daemon state once the tunnel is actually
	// up, so "Connected in background" is never printed for a connection
	// that never comes up.
	if err := s.waitForConnected(mgmt, connectStartupDeadline); err != nil {
		s.mu.Lock()
		s.mgmt = nil
		s.mu.Unlock()

		if errors.Is(err, errTunnelStopped) {
			// The user asked to disconnect while the tunnel was still
			// coming up: report a clean stop, not an error.
			_ = mgmt.Disconnect()
			_ = mgmt.Close()
			_ = cmd.Wait()
			return nil
		}

		_ = mgmt.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}

	if err := daemon.Save(daemon.State{
		PID:         os.Getpid(),
		ControlAddr: s.control.Addr(),
		HostName:    server.HostName,
		IPAddr:      server.IPAddr,
		CountryLong: server.CountryLong,
		StartedAt:   startedAt,
	}); err != nil {
		_ = mgmt.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}

	log.Info().Msgf("Connected in background to %s (%s) in %s", server.HostName, server.IPAddr, server.CountryLong)

	waitErr := cmd.Wait()

	s.mu.Lock()
	if s.mgmt != nil {
		_ = s.mgmt.Close()
		s.mgmt = nil
	}
	s.mu.Unlock()

	return waitErr
}

// reserveLoopbackAddr picks a free loopback TCP port by opening then
// immediately closing a listener, so the caller can hand the address to
// a separate process (openvpn) rather than a listener it can't pass on.
func reserveLoopbackAddr() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		return "", err
	}
	return addr, nil
}

// waitForManagement polls addr until OpenVPN's management interface
// accepts a connection, a stop is requested, or timeout elapses. It is a
// method (rather than a plain function) so the serve path — where a
// disconnect arrives over HTTP while openvpn is still starting — answers
// promptly instead of waiting out the full timeout.
func (s *supervisor) waitForManagement(addr string, timeout time.Duration) (*daemon.Management, error) {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		stopping := s.stopping
		s.mu.Unlock()
		if stopping {
			return nil, errTunnelStopped
		}
		mgmt, err := daemon.DialManagement(addr, time.Second)
		if err == nil {
			return mgmt, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for openvpn management interface: %w", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// errTunnelStopped is returned by waitForConnected when a stop was
// requested while openvpn was still bringing the tunnel up.
var errTunnelStopped = errors.New("disconnect requested while connecting")

// waitForConnected blocks until openvpn's management interface reports
// state CONNECTED, the tunnel process exits (State() then fails because
// the management socket closes), a disconnect is requested, or timeout
// elapses. It is what makes connectOnce fail fast when openvpn dies during
// the handshake instead of publishing a connection that never came up.
func (s *supervisor) waitForConnected(mgmt *daemon.Management, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		stopping := s.stopping
		s.mu.Unlock()
		if stopping {
			return errTunnelStopped
		}

		s.stateMu.Lock()
		state, err := mgmt.State()
		s.stateMu.Unlock()
		if err != nil {
			s.mu.Lock()
			stopping := s.stopping
			s.mu.Unlock()
			if stopping {
				// handleStop signaled openvpn mid-query; the socket
				// closing is the result of the stop, not a failure.
				return errTunnelStopped
			}
			return fmt.Errorf("management interface closed before the tunnel was up: %w", err)
		}
		if state == "CONNECTED" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tunnel not up within %s (state: %s)", timeout, state)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// handleStatus answers a STATUS control request with the supervisor's
// current view of the connection.
func (s *supervisor) handleStatus() (daemon.Snapshot, error) {
	s.mu.Lock()
	mgmt := s.mgmt
	server := s.server
	startedAt := s.startedAt
	s.mu.Unlock()

	state := "CONNECTING"
	if mgmt != nil {
		s.stateMu.Lock()
		st, err := mgmt.State()
		s.stateMu.Unlock()
		if err == nil {
			state = st
		}
	}

	return daemon.Snapshot{
		State:       state,
		HostName:    server.HostName,
		IPAddr:      server.IPAddr,
		CountryLong: server.CountryLong,
		StartedAt:   startedAt,
		PID:         os.Getpid(),
	}, nil
}

// handleStop answers a STOP control request: it marks the supervisor as
// stopping (so the reconnect loop in run() won't respawn openvpn) and,
// if openvpn is currently connected, asks it to exit cleanly. run()
// observes s.stopping either when connectOnce's cmd.Wait() returns or,
// if no openvpn is running yet, on its next loop iteration.
func (s *supervisor) handleStop() {
	s.mu.Lock()
	s.stopping = true
	mgmt := s.mgmt
	s.mu.Unlock()

	if mgmt != nil {
		_ = mgmt.Disconnect()
	}
}
