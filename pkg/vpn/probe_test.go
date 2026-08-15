package vpn

import (
	"context"
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// fakeOpenVPN writes an executable stub that emits the given marker line
// (e.g. "PUSH_REPLY,..." or "AUTH_FAILED") and then sleeps, and points the
// probe at it via VPNGATE_OPENVPN_BIN.
func fakeOpenVPN(t *testing.T, marker string) (cleanup func()) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("fake openvpn stub requires a POSIX shell")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "openvpn")
	script := "#!/bin/sh\n"
	if marker != "" {
		script += "echo '" + marker + "'\n"
	}
	script += "sleep 30\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	old := os.Getenv(probeOpenVPNBinEnv)
	if err := os.Setenv(probeOpenVPNBinEnv, bin); err != nil {
		t.Fatal(err)
	}
	return func() {
		if old == "" {
			_ = os.Unsetenv(probeOpenVPNBinEnv)
		} else {
			_ = os.Setenv(probeOpenVPNBinEnv, old)
		}
	}
}

// fakeOpenVPNSeq writes an executable stub that emits each line in order,
// sleeping delay between lines, so tests can model the real OpenVPN
// handshake ordering (e.g. "Peer Connection Initiated" followed later by
// "AUTH_FAILED").
func fakeOpenVPNSeq(t *testing.T, delay time.Duration, lines ...string) (cleanup func()) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("fake openvpn stub requires a POSIX shell")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "openvpn")
	script := "#!/bin/sh\n"
	for _, l := range lines {
		script += "echo '" + l + "'\n"
		script += "sleep " + strconv.FormatFloat(delay.Seconds(), 'f', 1, 64) + "\n"
	}
	script += "sleep 30\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	old := os.Getenv(probeOpenVPNBinEnv)
	if err := os.Setenv(probeOpenVPNBinEnv, bin); err != nil {
		t.Fatal(err)
	}
	return func() {
		if old == "" {
			_ = os.Unsetenv(probeOpenVPNBinEnv)
		} else {
			_ = os.Setenv(probeOpenVPNBinEnv, old)
		}
	}
}

// serverWithConfig builds a Server whose config points at host:port.
func serverWithConfig(host, port string) Server {
	cfg := "client\nremote " + host + " " + port + " tcp\ntls-client\n"
	return Server{
		HostName:          "test-server",
		IPAddr:            host,
		OpenVpnConfigData: base64.StdEncoding.EncodeToString([]byte(cfg)),
	}
}

// localTCPListener returns an address (host, port) that accepts TCP
// connections without the test needing to accept them.
func localTCPListener(t *testing.T) (host string, port string, close func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", strconv.Itoa(tcpAddr.Port), func() { _ = ln.Close() }
}

func TestParseRemoteFromConfig(t *testing.T) {
	cfg := base64.StdEncoding.EncodeToString([]byte("client\nremote example.com 995 tcp\n"))
	host, port, ok := ParseRemoteFromConfig(cfg)
	if !ok {
		t.Fatal("expected ok")
	}
	if host != "example.com" || port != "995" {
		t.Fatalf("got host=%q port=%q", host, port)
	}

	if _, _, ok := ParseRemoteFromConfig("not-base64!"); ok {
		t.Fatal("expected ok=false for invalid base64")
	}
}

func TestProbeServerUnreachable(t *testing.T) {
	// Bind then close a listener so the port is guaranteed refused.
	host, port, close := localTCPListener(t)
	close()

	server := serverWithConfig(host, port)
	result := ProbeServer(context.Background(), &server, 2*time.Second)
	if result.Status != ProbeUnreachable {
		t.Fatalf("expected unreachable, got %v", result.Status)
	}
}

func TestProbeServerWorking(t *testing.T) {
	cleanup := fakeOpenVPN(t, "PUSH_REPLY,route 0.0.0.0 0.0.0.0")
	defer cleanup()

	host, port, close := localTCPListener(t)
	defer close()

	server := serverWithConfig(host, port)
	result := ProbeServer(context.Background(), &server, 5*time.Second)
	if result.Status != ProbeWorking {
		t.Fatalf("expected working, got %v (detail: %s)", result.Status, result.Detail)
	}
	if result.LatencyMs <= 0 {
		t.Fatalf("expected positive latency, got %d", result.LatencyMs)
	}
}

func TestProbeServerAuthFailed(t *testing.T) {
	cleanup := fakeOpenVPN(t, "AUTH_FAILED")
	defer cleanup()

	host, port, close := localTCPListener(t)
	defer close()

	server := serverWithConfig(host, port)
	result := ProbeServer(context.Background(), &server, 5*time.Second)
	if result.Status != ProbeAuthFailed {
		t.Fatalf("expected auth_failed, got %v (detail: %s)", result.Status, result.Detail)
	}
}

func TestProbeServerMissingBinary(t *testing.T) {
	old := os.Getenv(probeOpenVPNBinEnv)
	_ = os.Setenv(probeOpenVPNBinEnv, "/nonexistent/openvpn")
	defer func() {
		if old == "" {
			_ = os.Unsetenv(probeOpenVPNBinEnv)
		} else {
			_ = os.Setenv(probeOpenVPNBinEnv, old)
		}
	}()

	host, port, close := localTCPListener(t)
	defer close()

	server := serverWithConfig(host, port)
	result := ProbeServer(context.Background(), &server, 2*time.Second)
	if result.Status != ProbeError {
		t.Fatalf("expected error, got %v", result.Status)
	}
}

func TestProbeServersConcurrency(t *testing.T) {
	cleanup := fakeOpenVPN(t, "PUSH_REPLY,route 0.0.0.0 0.0.0.0")
	defer cleanup()

	host, port, close := localTCPListener(t)
	defer close()

	servers := []Server{
		serverWithConfig(host, port),
		serverWithConfig(host, port),
		serverWithConfig(host, port),
	}
	for i := range servers {
		servers[i].HostName = "srv-" + strconv.Itoa(i)
	}

	results := ProbeServers(context.Background(), servers, 2, 5*time.Second)
	if len(results) != len(servers) {
		t.Fatalf("expected %d results, got %d", len(servers), len(results))
	}
	for _, r := range results {
		if r.Status != ProbeWorking {
			t.Fatalf("expected all working, got %v", r.Status)
		}
	}
}

func TestBestWorkingServer(t *testing.T) {
	a := serverWithConfig("1.1.1.1", "443")
	a.HostName = "a"
	b := serverWithConfig("2.2.2.2", "443")
	b.HostName = "b"
	c := serverWithConfig("3.3.3.3", "443")
	c.HostName = "c"

	results := map[string]ProbeResult{
		"a": {Status: ProbeWorking, LatencyMs: 300},
		"b": {Status: ProbeWorking, LatencyMs: 50},
		"c": {Status: ProbeAuthFailed},
	}

	best, err := BestWorkingServer([]Server{a, b, c}, results)
	if err != nil {
		t.Fatal(err)
	}
	if best.HostName != "b" {
		t.Fatalf("expected b, got %s", best.HostName)
	}

	_, err = BestWorkingServer([]Server{a, b, c}, map[string]ProbeResult{})
	if err == nil {
		t.Fatal("expected error when no working servers")
	}
}

// TestProbeServerTlsInitNotWorking verifies that reaching a TLS session
// ("Peer Connection Initiated") alone is not sufficient to report a relay as
// working; a full/maintenance relay still rejects credentials with AUTH_FAILED
// afterward, so the probe must classify it as not-working (timeout here).
func TestProbeServerTlsInitNotWorking(t *testing.T) {
	cleanup := fakeOpenVPN(t, "Peer Connection Initiated with [AF_INET]1.2.3.4:443")
	defer cleanup()

	host, port, close := localTCPListener(t)
	defer close()

	server := serverWithConfig(host, port)
	result := ProbeServer(context.Background(), &server, 3*time.Second)
	if result.Status == ProbeWorking {
		t.Fatalf("expected NOT working for a relay that only reaches TLS init, got %v", result.Status)
	}
}

// TestProbeServerInitThenAuthFailed verifies the real failure mode: a relay
// completes the TLS handshake ("Peer Connection Initiated") and then rejects
// credentials with AUTH_FAILED. The probe must classify it as auth_failed, not
// working, so the TUI/--best never steer into a full relay.
func TestProbeServerInitThenAuthFailed(t *testing.T) {
	cleanup := fakeOpenVPNSeq(t, 1*time.Second,
		"Peer Connection Initiated with [AF_INET]1.2.3.4:443",
		"AUTH_FAILED",
	)
	defer cleanup()

	host, port, close := localTCPListener(t)
	defer close()

	server := serverWithConfig(host, port)
	result := ProbeServer(context.Background(), &server, 5*time.Second)
	if result.Status != ProbeAuthFailed {
		t.Fatalf("expected auth_failed (relay full), got %v (detail: %s)", result.Status, result.Detail)
	}
}
