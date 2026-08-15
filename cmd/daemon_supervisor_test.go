package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/davegallant/vpngate/pkg/daemon"
)

func TestReserveLoopbackAddr(t *testing.T) {
	addr, err := reserveLoopbackAddr()
	assert.NoError(t, err)
	assert.NotEmpty(t, addr)

	// The port must be free immediately afterward.
	ln, err := net.Listen("tcp", addr)
	assert.NoError(t, err)
	defer func() { _ = ln.Close() }()
}

func TestWaitForManagementSucceeds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer func() { _ = ln.Close() }()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = conn.Write([]byte(">INFO:OpenVPN Management Interface Version 5\n"))
		// Keep the connection open for State()/Disconnect() calls the
		// caller might make; just block until the test ends.
		buf := make([]byte, 1)
		_, _ = conn.Read(buf)
	}()

	mgmt, err := waitForManagement(ln.Addr().String(), time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, mgmt)
	defer func() { _ = mgmt.Close() }()
}

func TestWaitForManagementTimesOut(t *testing.T) {
	_, err := waitForManagement("127.0.0.1:1", 300*time.Millisecond)
	assert.Error(t, err)
}

// fakeManagementServer runs a fake OpenVPN management interface on a
// loopback listener and returns its address. Each "state" command is
// answered with the next entry of states; once the list is exhausted the
// last state repeats.
func fakeManagementServer(t *testing.T, states ...string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = conn.Write([]byte(">INFO:OpenVPN Management Interface Version 5\n"))

		sc := bufio.NewScanner(conn)
		i := 0
		for sc.Scan() {
			if !strings.HasPrefix(sc.Text(), "state") {
				continue
			}
			if len(states) == 0 {
				return
			}
			if i >= len(states) {
				i = len(states) - 1
			}
			_, _ = fmt.Fprintf(conn, "150,%s,,,,\nEND\n", states[i])
			i++
		}
	}()

	return ln.Addr().String()
}

// TestWaitForConnectedSuccess verifies waitForConnected returns once the
// management interface reports state CONNECTED.
func TestWaitForConnectedSuccess(t *testing.T) {
	mgmt, err := daemon.DialManagement(fakeManagementServer(t, "CONNECTING", "CONNECTED"), time.Second)
	assert.NoError(t, err)
	defer func() { _ = mgmt.Close() }()

	err = (&supervisor{}).waitForConnected(mgmt, 3*time.Second)
	assert.NoError(t, err)
}

// TestWaitForConnectedTimeout verifies waitForConnected gives up after the
// timeout when the tunnel never reaches CONNECTED.
func TestWaitForConnectedTimeout(t *testing.T) {
	mgmt, err := daemon.DialManagement(fakeManagementServer(t, "WAIT"), time.Second)
	assert.NoError(t, err)
	defer func() { _ = mgmt.Close() }()

	start := time.Now()
	err = (&supervisor{}).waitForConnected(mgmt, 300*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel not up within")
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
}

// TestWaitForConnectedStopping verifies waitForConnected returns
// errTunnelStopped when a disconnect is requested mid-connect.
func TestWaitForConnectedStopping(t *testing.T) {
	mgmt, err := daemon.DialManagement(fakeManagementServer(t, "WAIT"), time.Second)
	assert.NoError(t, err)
	defer func() { _ = mgmt.Close() }()

	err = (&supervisor{stopping: true}).waitForConnected(mgmt, 3*time.Second)
	assert.ErrorIs(t, err, errTunnelStopped)
}

// TestWaitForConnectedManagementClosed verifies a tunnel that dies before
// reaching CONNECTED (the management socket closes) is reported as an
// error rather than hanging.
func TestWaitForConnectedManagementClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer func() { _ = ln.Close() }()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = conn.Write([]byte(">INFO:OpenVPN Management Interface Version 5\n"))
	}()

	mgmt, err := daemon.DialManagement(ln.Addr().String(), time.Second)
	assert.NoError(t, err)
	defer func() { _ = mgmt.Close() }()

	err = (&supervisor{}).waitForConnected(mgmt, 3*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "management interface closed")
	assert.False(t, errors.Is(err, errTunnelStopped), "a closed socket with no stop requested is not a clean stop")
}
