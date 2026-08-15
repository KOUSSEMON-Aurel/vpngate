package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/davegallant/vpngate/pkg/vpn"
)

// fakeOpenVPNBin writes a fake openvpn executable into a temp dir and
// prepends that dir to PATH so LookPath resolves it. The fake behavior is
// selected by the server's HostName:
//   - ok:    prints "Initialization Sequence Completed" then sleeps forever
//   - auth:  prints an AUTH_FAILED control message then exits 1
//   - slow:  prints nothing and sleeps 300s (never initializes)
//   - crash: exits 1 immediately with an error line
func fakeOpenVPNBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
prev=
for arg in "$@"; do
  if [ "$prev" = "--config" ]; then cfg="$arg"; fi
  prev="$arg"
done
if [ -f "$cfg" ]; then cfg="$(cat "$cfg")"; fi
case "$cfg" in
  *ok*)
    echo "TLS: tls_multi_process: initial untrusted session promoted to trusted"
    echo "Initialization Sequence Completed"
    sleep 300 &
    wait
    ;;
  *auth*)
    echo "AUTH: Received control message: AUTH_FAILED"
    echo "SIGTERM[soft,auth-failure] received, process exiting"
    exit 1
    ;;
  *slow*)
    sleep 300 &
    wait
    ;;
  *)
    echo "ERROR: cannot open TUN/TAP dev /dev/net/tun"
    exit 1
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "openvpn"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return dir
}

// fakeServer builds a Server whose config selects the fake openvpn
// behavior above via the HostName embedded in a config comment.
func fakeServer(name string) vpn.Server {
	cfg := "client\ndev tun\nremote 10.0.0.1 1194 # " + name + " ):\n"
	return vpn.Server{
		HostName:          name,
		IPAddr:            "10.0.0.1",
		CountryLong:       "Japan",
		OpenVpnConfigData: base64.StdEncoding.EncodeToString([]byte(cfg)),
	}
}

// TestConnectWithRetryFirstSucceeds verifies the preferred relay is used
// directly when it initializes.
func TestConnectWithRetryFirstSucceeds(t *testing.T) {
	fakeOpenVPNBin(t)
	var emitted []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connectWithRetry(ctx, []vpn.Server{fakeServer("ok"), fakeServer("crash")}, func(s string) { emitted = append(emitted, s) }, nil)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(emitted, "\n"), "Initialization Sequence Completed") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err, "user stop should not report an error")
	case <-time.After(5 * time.Second):
		t.Fatal("connectWithRetry did not return after cancel")
	}
	assert.NotContains(t, strings.Join(emitted, "\n"), "trying next relay", "no retry expected when the first relay works")
}

// TestConnectWithRetryAuthThenSuccess verifies a relay that refuses with
// AUTH_FAILED is skipped in favor of the next candidate.
func TestConnectWithRetryAuthThenSuccess(t *testing.T) {
	fakeOpenVPNBin(t)
	connectStartupDeadline = 2 * time.Second
	defer func() { connectStartupDeadline = 40 * time.Second }()

	var emitted []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connectWithRetry(ctx, []vpn.Server{fakeServer("auth"), fakeServer("ok")}, func(s string) { emitted = append(emitted, s) }, nil)
	}()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(emitted, "\n"), "Initialization Sequence Completed") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(strings.Join(emitted, "\n"), "Initialization Sequence Completed") {
		t.Fatal("second relay never initialized")
	}
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("connectWithRetry did not return after cancel")
	}
	assert.Contains(t, strings.Join(emitted, "\n"), "trying next relay", "expected a retry note after AUTH_FAILED")
}

// TestConnectWithRetryAllFail verifies an aggregate error when every
// candidate fails.
func TestConnectWithRetryAllFail(t *testing.T) {
	fakeOpenVPNBin(t)
	connectStartupDeadline = 2 * time.Second
	defer func() { connectStartupDeadline = 40 * time.Second }()

	var emitted []string
	err := connectWithRetry(context.Background(), []vpn.Server{fakeServer("auth"), fakeServer("crash")}, func(s string) { emitted = append(emitted, s) }, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all 2 attempted relays failed")
	assert.Contains(t, err.Error(), "refused the connection")
	assert.Contains(t, strings.Join(emitted, "\n"), "trying next relay")
}

// TestConnectWithRetrySlowThenSuccess verifies the startup deadline
// abandons a relay that never initializes.
func TestConnectWithRetrySlowThenSuccess(t *testing.T) {
	fakeOpenVPNBin(t)
	connectStartupDeadline = 2 * time.Second
	defer func() { connectStartupDeadline = 40 * time.Second }()

	var emitted []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- connectWithRetry(ctx, []vpn.Server{fakeServer("slow"), fakeServer("ok")}, func(s string) { emitted = append(emitted, s) }, nil)
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(emitted, "\n"), "Initialization Sequence Completed") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("connectWithRetry did not return after cancel")
	}
	assert.Contains(t, strings.Join(emitted, "\n"), "trying next relay")
}

// TestOrderedCandidates verifies candidate ordering: preferred first,
// then working-by-latency, then the rest by score.
func TestOrderedCandidates(t *testing.T) {
	servers := []vpn.Server{
		{HostName: "a", Score: 1},
		{HostName: "b", Score: 9},
		{HostName: "c", Score: 5},
		{HostName: "d", Score: 3},
	}
	results := map[string]vpn.ProbeResult{
		"c": {Status: vpn.ProbeWorking, LatencyMs: 100},
		"b": {Status: vpn.ProbeWorking, LatencyMs: 50},
	}

	got := orderedCandidates(servers, vpn.Server{HostName: "d"}, results)
	var names []string
	for _, s := range got {
		names = append(names, s.HostName)
	}
	assert.Equal(t, []string{"d", "b", "c", "a"}, names, "preferred, then working by latency, then rest by score")
}

// TestConnectAttemptAuthFailed verifies the AUTH_FAILED marker maps to the
// authFailed result.
func TestConnectAttemptAuthFailed(t *testing.T) {
	fakeOpenVPNBin(t)
	connectStartupDeadline = 2 * time.Second
	defer func() { connectStartupDeadline = 40 * time.Second }()

	res := connectAttempt(context.Background(), fakeServer("auth"), nil, nil)
	assert.True(t, res.authFailed)
	assert.Error(t, res.err)
	if !errors.Is(res.err, errors.New("relay refused the connection")) {
		assert.Equal(t, "relay refused the connection", res.err.Error())
	}
}

// TestConnectAttemptCrashFailsFast verifies an openvpn that exits before
// initializing the tunnel is reported immediately instead of being held
// until the startup deadline.
func TestConnectAttemptCrashFailsFast(t *testing.T) {
	fakeOpenVPNBin(t)
	connectStartupDeadline = 5 * time.Second
	defer func() { connectStartupDeadline = 40 * time.Second }()

	start := time.Now()
	res := connectAttempt(context.Background(), fakeServer("crash"), nil, nil)
	elapsed := time.Since(start)

	assert.Error(t, res.err)
	assert.False(t, res.authFailed)
	assert.Contains(t, res.err.Error(), "cannot open TUN/TAP", "expected the openvpn error to surface")
	if elapsed > 3*time.Second {
		t.Fatalf("crash attempt took too long to fail: %s (deadline was %s)", elapsed, connectStartupDeadline)
	}
}

// TestPrivilegeHintTunModule verifies the hint suggests loading the tun
// kernel module when openvpn cannot open /dev/net/tun.
func TestPrivilegeHintTunModule(t *testing.T) {
	hint := privilegeHint(errors.New("exit status 1: ERROR: Cannot open TUN/TAP dev /dev/net/tun: No such device (errno=19)"))
	assert.Contains(t, hint, "sudo modprobe tun")
}

// TestPrivilegeHintPermissions verifies the hint suggests re-running with
// sudo when openvpn reports insufficient privileges.
func TestPrivilegeHintPermissions(t *testing.T) {
	hint := privilegeHint(errors.New("ERROR: Cannot open TUN/TAP dev /dev/net/tun: Operation not permitted (errno=1)"))
	assert.Contains(t, hint, "sudo vpngate connect")
}

// TestConnectWithRetryCancelDuringAttempt verifies that canceling while a
// relay is still connecting returns promptly (and never panics on a send
// racing with the output stream being closed).
func TestConnectWithRetryCancelDuringAttempt(t *testing.T) {
	fakeOpenVPNBin(t)
	connectStartupDeadline = 60 * time.Second
	defer func() { connectStartupDeadline = 40 * time.Second }()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := connectWithRetry(ctx, []vpn.Server{fakeServer("slow")}, func(string) {}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("cancel took too long: %s", elapsed)
	}
}

// TestKeepAliveHealthyTunnelStaysUp verifies a tunnel passing its health
// checks is never killed by the health monitor.
func TestKeepAliveHealthyTunnelStaysUp(t *testing.T) {
	oldInterval := tunnelHealthInterval
	oldCheck := tunnelHealthCheck
	oldGrace := tunnelHealthGrace
	t.Cleanup(func() {
		tunnelHealthInterval = oldInterval
		tunnelHealthCheck = oldCheck
		tunnelHealthGrace = oldGrace
	})
	tunnelHealthInterval = 20 * time.Millisecond
	tunnelHealthGrace = 0
	tunnelHealthCheck = func() error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		keepTunnelAlive(ctx, cancel, nil, &tunnelHealth{})
		close(done)
	}()

	time.Sleep(120 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatal("healthy tunnel was killed by the health monitor")
	}
	cancel()
	<-done
}

// TestKeepAliveDeadTunnelCancels verifies the tunnel is torn down and the
// reconnecting marker emitted after tunnelHealthMaxFails failed checks.
func TestKeepAliveDeadTunnelCancels(t *testing.T) {
	oldInterval := tunnelHealthInterval
	oldMax := tunnelHealthMaxFails
	oldCheck := tunnelHealthCheck
	oldGrace := tunnelHealthGrace
	t.Cleanup(func() {
		tunnelHealthInterval = oldInterval
		tunnelHealthMaxFails = oldMax
		tunnelHealthCheck = oldCheck
		tunnelHealthGrace = oldGrace
	})
	tunnelHealthInterval = 20 * time.Millisecond
	tunnelHealthGrace = 0
	tunnelHealthMaxFails = 2
	tunnelHealthCheck = func() error { return errors.New("no egress") }

	ctx, cancel := context.WithCancel(context.Background())
	var lines []string
	keepTunnelAlive(ctx, cancel, func(line string) { lines = append(lines, line) }, &tunnelHealth{})

	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timed out: health monitor never canceled the tunnel")
		}
		if ctx.Err() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "tunnel appears dead") {
		t.Fatalf("expected dead-tunnel marker, got %q", got)
	}
}

// TestConnectAttemptConfigBlackholesIPv6 verifies the client config written
// for every attempt forces IPv6 into the tunnel so no traffic can bypass it.
func TestConnectAttemptConfigBlackholesIPv6(t *testing.T) {
	// Isolate the temp dir so stale configs left behind by real connect
	// sessions (they linger in /tmp after a hard kill) can never be
	// mistaken for this attempt's freshly written config.
	t.Setenv("TMPDIR", t.TempDir())
	fakeOpenVPNBin(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- connectServer(ctx, fakeServer("ok"), io.Discard) }()

	// The fake openvpn sleeps forever, so the temp config exists while
	// the attempt is live; snap it (the newest match — stale files from
	// hard-killed sessions can linger in /tmp) before canceling.
	var cfg string
	deadline := time.Now().Add(5 * time.Second)
	for {
		var newest string
		var newestMod time.Time
		matches, _ := filepath.Glob(os.TempDir() + "/vpngate-openvpn-config-*")
		for _, m := range matches {
			if fi, err := os.Stat(m); err == nil && fi.ModTime().After(newestMod) {
				newest, newestMod = m, fi.ModTime()
			}
		}
		if newest != "" {
			if raw, err := os.ReadFile(newest); err == nil {
				cfg = string(raw)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the client config")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	// The black-hole directives must match the host's IPv6 reality: a
	// host with a default IPv6 route gets route-ipv6, an IPv6-less host
	// must not carry a directive openvpn would reject with a warning.
	if hostRoutesIPv6() {
		if !strings.Contains(cfg, "route-ipv6 ::/0") {
			t.Fatalf("config missing IPv6 black-hole on IPv6 host:\n%s", cfg)
		}
	} else if strings.Contains(cfg, "route-ipv6") {
		t.Fatalf("config contains IPv6 black-hole on IPv6-less host:\n%s", cfg)
	}
}

// TestTunnelHealthCheckAnyEndpoint verifies the tunnel is judged alive as
// soon as any probe endpoint answers, mirroring the partial egress seen on
// real relays: one endpoint unreachable must not kill a tunnel another
// endpoint proves working.
func TestTunnelHealthCheckAnyEndpoint(t *testing.T) {
	// One working endpoint among dead ones: the check must pass even
	// though every other probe fails. The dead endpoint's handler blocks
	// until the request context expires (the shared probe timeout), then
	// returns so its httptest server can shut down cleanly.
	var dead *httptest.Server
	dead = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer dead.Close()

	alive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer alive.Close()

	oldEndpoints := tunnelHealthEndpoints
	oldCheck := tunnelHealthCheck
	oldTimeout := tunnelHealthTimeout
	t.Cleanup(func() {
		tunnelHealthEndpoints = oldEndpoints
		tunnelHealthCheck = oldCheck
		tunnelHealthTimeout = oldTimeout
	})
	tunnelHealthEndpoints = []string{dead.URL, alive.URL}
	tunnelHealthTimeout = 300 * time.Millisecond
	tunnelHealthCheck = realTunnelHealthCheck

	if err := tunnelHealthCheck(); err != nil {
		t.Fatalf("tunnel judged dead with a working endpoint: %v", err)
	}
}

// TestTunnelHealthCheckAllDead verifies a tunnel fails the check only when
// every endpoint is unreachable.
func TestTunnelHealthCheckAllDead(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer dead.Close()

	oldEndpoints := tunnelHealthEndpoints
	oldCheck := tunnelHealthCheck
	oldTimeout := tunnelHealthTimeout
	t.Cleanup(func() {
		tunnelHealthEndpoints = oldEndpoints
		tunnelHealthCheck = oldCheck
		tunnelHealthTimeout = oldTimeout
	})
	tunnelHealthEndpoints = []string{dead.URL}
	tunnelHealthTimeout = 300 * time.Millisecond
	tunnelHealthCheck = realTunnelHealthCheck

	if err := tunnelHealthCheck(); err == nil {
		t.Fatal("tunnel judged alive with every endpoint unreachable")
	}
}

// TestKeepAlivePausedNeverCancels verifies a paused watchdog skips probes
// and accumulates no failures, so a tunnel whose egress is flaky is never
// dropped by it while the pause is on.
func TestKeepAlivePausedNeverCancels(t *testing.T) {
	oldInterval := tunnelHealthInterval
	oldMax := tunnelHealthMaxFails
	oldCheck := tunnelHealthCheck
	oldGrace := tunnelHealthGrace
	t.Cleanup(func() {
		tunnelHealthInterval = oldInterval
		tunnelHealthMaxFails = oldMax
		tunnelHealthCheck = oldCheck
		tunnelHealthGrace = oldGrace
	})
	tunnelHealthInterval = 20 * time.Millisecond
	tunnelHealthGrace = 0
	tunnelHealthMaxFails = 1
	tunnelHealthCheck = func() error { return errors.New("no egress") }

	var pause atomic.Bool
	pause.Store(true)
	health := &tunnelHealth{pause: &pause}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		keepTunnelAlive(ctx, cancel, nil, health)
		close(done)
	}()

	time.Sleep(120 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatal("paused watchdog canceled the tunnel")
	}
	cancel()
	<-done
}

// TestKeepAliveGraceWindowSkipsEarlyChecks verifies no health check runs
// during the grace period after the tunnel comes up.
func TestKeepAliveGraceWindowSkipsEarlyChecks(t *testing.T) {
	oldInterval := tunnelHealthInterval
	oldGrace := tunnelHealthGrace
	oldCheck := tunnelHealthCheck
	t.Cleanup(func() {
		tunnelHealthInterval = oldInterval
		tunnelHealthGrace = oldGrace
		tunnelHealthCheck = oldCheck
	})
	tunnelHealthInterval = 20 * time.Millisecond
	tunnelHealthGrace = 100 * time.Millisecond
	var checks atomic.Int64
	tunnelHealthCheck = func() error { checks.Add(1); return nil }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		keepTunnelAlive(ctx, cancel, nil, &tunnelHealth{})
		close(done)
	}()

	time.Sleep(60 * time.Millisecond)
	if got := checks.Load(); got != 0 {
		t.Fatalf("health check ran during the grace window (%d checks)", got)
	}
	cancel()
	<-done
}
