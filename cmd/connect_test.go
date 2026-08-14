package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		done <- connectWithRetry(ctx, []vpn.Server{fakeServer("ok"), fakeServer("crash")}, func(s string) { emitted = append(emitted, s) })
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
		done <- connectWithRetry(ctx, []vpn.Server{fakeServer("auth"), fakeServer("ok")}, func(s string) { emitted = append(emitted, s) })
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
	err := connectWithRetry(context.Background(), []vpn.Server{fakeServer("auth"), fakeServer("crash")}, func(s string) { emitted = append(emitted, s) })
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
		done <- connectWithRetry(ctx, []vpn.Server{fakeServer("slow"), fakeServer("ok")}, func(s string) { emitted = append(emitted, s) })
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

	res := connectAttempt(context.Background(), fakeServer("auth"), nil)
	assert.True(t, res.authFailed)
	assert.Error(t, res.err)
	if !errors.Is(res.err, errors.New("relay refused the connection")) {
		assert.Equal(t, "relay refused the connection", res.err.Error())
	}
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
	err := connectWithRetry(ctx, []vpn.Server{fakeServer("slow")}, func(string) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("cancel took too long: %s", elapsed)
	}
}
