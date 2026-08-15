package vpn

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ProbeStatus describes the verified usability of a VPN server.
type ProbeStatus string

const (
	// ProbeUnknown is the state before a server has been probed.
	ProbeUnknown ProbeStatus = "unknown"
	// ProbeChecking means a probe is currently in flight for the server.
	ProbeChecking ProbeStatus = "checking"
	// ProbeWorking means a real OpenVPN connection reached PUSH_REPLY (the
	// server accepted the session and pushed its config), so the server is
	// actually usable. A mere TLS "Peer Connection Initiated" is NOT enough
	// because a full/maintenance relay still rejects credentials seconds
	// later with AUTH_FAILED.
	ProbeWorking ProbeStatus = "working"
	// ProbeAuthFailed means the server accepted the connection but refused
	// the OpenVPN credentials (typically a full or maintenance relay).
	ProbeAuthFailed ProbeStatus = "auth_failed"
	// ProbeUnreachable means the TCP port is refused or filtered.
	ProbeUnreachable ProbeStatus = "unreachable"
	// ProbeTimeout means TCP connected but the OpenVPN handshake did not
	// complete within the probe timeout.
	ProbeTimeout ProbeStatus = "timeout"
	// ProbeError means the probe itself failed (missing binary, bad config).
	ProbeError ProbeStatus = "error"
)

// ProbeResult is the outcome of verifying a single server.
type ProbeResult struct {
	Status    ProbeStatus
	LatencyMs int
	Detail    string
}

// probeOpenVPNBin is the openvpn binary used for probes. It can be
// overridden via the VPNGATE_OPENVPN_BIN environment variable (tests use
// this to substitute a fake binary).
var probeOpenVPNBin = "openvpn"
var probeOpenVPNBinEnv = "VPNGATE_OPENVPN_BIN"

// ParseRemoteFromConfig extracts the first `remote host port` line from a
// base64-encoded OpenVPN config.
func ParseRemoteFromConfig(base64Data string) (host string, port string, ok bool) {
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", "", false
	}

	for _, line := range strings.Split(string(decoded), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "remote ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[1], parts[2], true
			}
			if len(parts) >= 2 {
				return parts[1], "443", true
			}
		}
	}

	return "", "", false
}

// ProbeServer verifies whether a server is actually usable by running a
// real OpenVPN handshake probe. For TCP relays the port is first cheaply
// dialed so unreachable hosts fail fast without spawning a process; UDP
// relays skip that check since dialing a UDP-only port over TCP would
// always fail.
func ProbeServer(ctx context.Context, server *Server, timeout time.Duration) ProbeResult {
	host, port, ok := ParseRemoteFromConfig(server.OpenVpnConfigData)
	if !ok {
		host = server.IPAddr
		port = "443"
	}

	if protocolFromConfig(server.OpenVpnConfigData) == "tcp" && !reachableTCP(host, port, timeout) {
		return ProbeResult{
			Status: ProbeUnreachable,
			Detail: fmt.Sprintf("%s:%s not reachable", host, port),
		}
	}

	return probeOpenVPN(ctx, server, timeout)
}

// protocolFromConfig returns the transport protocol declared in an
// OpenVPN config ("tcp" or "udp"), defaulting to "udp" which is what
// OpenVPN itself assumes when no proto line is present. Both the `proto`
// directive and a trailing protocol on the `remote` line are honored.
func protocolFromConfig(base64Data string) string {
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "udp"
	}
	for _, line := range strings.Split(string(decoded), "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if strings.HasPrefix(line, "proto ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strings.TrimSuffix(fields[1], "\r")
			}
		}
		if strings.HasPrefix(line, "remote ") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				if proto := strings.TrimSuffix(fields[3], "\r"); proto == "tcp" || proto == "udp" {
					return proto
				}
			}
		}
	}
	return "udp"
}

// reachableTCP performs a cheap TCP dial so unreachable hosts fail fast
// without spawning a process.
func reachableTCP(host string, port string, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// probeOpenVPN runs the real openvpn binary against the server's config and
// reports working only once the server actually pushes its config
// (PUSH_REPLY / Initialization Sequence Completed). Reaching a TLS session
// alone is not sufficient because a full relay can still reject with
// AUTH_FAILED shortly afterward.
func probeOpenVPN(ctx context.Context, server *Server, timeout time.Duration) ProbeResult {
	config, err := base64.StdEncoding.DecodeString(server.OpenVpnConfigData)
	if err != nil {
		return ProbeResult{Status: ProbeError, Detail: "invalid base64 config"}
	}

	tmpfile, err := os.CreateTemp("", "vpngate-probe-*.ovpn")
	if err != nil {
		return ProbeResult{Status: ProbeError, Detail: err.Error()}
	}
	if _, err := tmpfile.Write(config); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return ProbeResult{Status: ProbeError, Detail: err.Error()}
	}
	_ = tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	bin := probeOpenVPNBin
	if env := os.Getenv(probeOpenVPNBinEnv); env != "" {
		bin = env
	}

	connectTimeout := int(timeout.Seconds())
	if connectTimeout < 1 {
		connectTimeout = 1
	}

	args := []string{
		"--config", tmpfile.Name(),
		"--dev", "null",
		"--ifconfig-noexec",
		"--route-noexec",
		"--connect-retry-max", "1",
		"--connect-retry", "1",
		"--connect-timeout", fmt.Sprint(connectTimeout),
		"--verb", "3",
	}

	// vpnbook configs carry a bare "auth-user-pass" directive that would
	// block on stdin waiting for credentials; pass the shared credentials
	// file explicitly so the probe can complete.
	if server.Source == SourceVpnbook {
		credsFile, err := vpnbookCredsFileFor()
		if err != nil {
			_ = os.Remove(tmpfile.Name())
			return ProbeResult{Status: ProbeError, Detail: "vpnbook credentials unavailable: " + err.Error()}
		}
		args = append(args, "--auth-user-pass", credsFile)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ProbeResult{Status: ProbeError, Detail: err.Error()}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ProbeResult{Status: ProbeError, Detail: err.Error()}
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return ProbeResult{Status: ProbeError, Detail: err.Error()}
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return scanProbeOutput(probeCtx, cmd, stdout, stderr, start)
}

// scanProbeOutput tails the combined openvpn output looking for the
// handshake markers and reports the first meaningful state. The process is
// always reaped before returning.
func scanProbeOutput(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.Reader, start time.Time) ProbeResult {
	lines := make(chan string, 256)
	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(stderr)
	go func() {
		wg.Wait()
		close(lines)
	}()

	waitDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waitDone)
	}()

	killAndReap := func() {
		_ = cmd.Process.Kill()
		<-waitDone
	}

	lastLine := ""
	for {
		select {
		case line, open := <-lines:
			if !open {
				if strings.Contains(lastLine, "AUTH_FAILED") {
					return ProbeResult{Status: ProbeAuthFailed, LatencyMs: int(time.Since(start).Milliseconds())}
				}
				return ProbeResult{Status: ProbeTimeout, Detail: strings.TrimSpace(lastLine)}
			}
			lastLine = line

			switch {
			case strings.Contains(line, "PUSH_REPLY"), strings.Contains(line, "Initialization Sequence Completed"):
				// The server accepted the session and pushed its config,
				// so the relay is genuinely usable.
				killAndReap()
				return ProbeResult{Status: ProbeWorking, LatencyMs: int(time.Since(start).Milliseconds())}
			case strings.Contains(line, "AUTH_FAILED"):
				killAndReap()
				return ProbeResult{Status: ProbeAuthFailed, LatencyMs: int(time.Since(start).Milliseconds())}
			}
			// "Peer Connection Initiated" is intentionally NOT treated as
			// working: it only means the TLS session is up, and a
			// full/maintenance relay still rejects credentials moments
			// later with AUTH_FAILED. Keep scanning so such relays are
			// reported as auth_failed (or timeout), never as working.
		case <-ctx.Done():
			// Kill and reap so no openvpn process is left behind, even
			// when the parent is cancelled mid-probe.
			killAndReap()
			return ProbeResult{Status: ProbeTimeout, Detail: "probe timed out"}
		}
	}
}

// ProbeServers probes servers concurrently with a bounded number of
// parallel probes and returns a map keyed by hostname. When onResult is
// provided it is invoked with each completed result as soon as it is ready
// so callers can stream progress live instead of waiting for the whole
// batch.
func ProbeServers(ctx context.Context, servers []Server, concurrency int, timeout time.Duration, onResult ...func(name string, res ProbeResult)) map[string]ProbeResult {
	results := make(map[string]ProbeResult, len(servers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	if concurrency < 1 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)

loop:
	for i := range servers {
		select {
		case <-ctx.Done():
			// Stop dispatching new probes once the context is cancelled;
			// in-flight probes finish and are cleaned up by their own
			// timeouts.
			break loop
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(s *Server) {
			defer wg.Done()
			defer func() { <-sem }()

			result := ProbeServer(ctx, s, timeout)

			mu.Lock()
			results[s.HostName] = result
			mu.Unlock()

			if len(onResult) > 0 {
				onResult[0](s.HostName, result)
			}

			if result.Status == ProbeWorking {
				log.Debug().Msgf("%s (%s) verified working (%dms)", s.HostName, s.IPAddr, result.LatencyMs)
			} else {
				log.Debug().Msgf("%s (%s) %s: %s", s.HostName, s.IPAddr, result.Status, result.Detail)
			}
		}(&servers[i])
	}

	wg.Wait()
	return results
}

// BestWorkingServer returns the working server with the lowest real
// latency, or an error when none of the servers verified as working.
func BestWorkingServer(servers []Server, results map[string]ProbeResult) (Server, error) {
	best := Server{}
	bestLatency := int(^uint(0) >> 1)
	found := false

	for _, s := range servers {
		r, ok := results[s.HostName]
		if !ok || r.Status != ProbeWorking {
			continue
		}
		if r.LatencyMs < bestLatency {
			best = s
			bestLatency = r.LatencyMs
			found = true
		}
	}

	if !found {
		return Server{}, errors.New("no working vpn servers found")
	}
	return best, nil
}
