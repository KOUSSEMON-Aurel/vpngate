package vpn

import (
	"context"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/davegallant/vpngate/pkg/exec"
)

// vpngateSSTPUser and vpngateSSTPPassword are the shared credentials VPN
// Gate publishes for every MS-SSTP relay. Like L2TP, they are public and
// fixed for the whole pool.
const (
	vpngateSSTPUser     = "vpn"
	vpngateSSTPPassword = "vpn"
)

// sstpClient connects to a vpngate relay through an MS-SSTP tunnel built
// from sstp-client (sstpc) + pppd. sstpc runs inside a generated wrapper
// script so the tunnel is one child process group that can be torn down
// together.
type sstpClient struct{}

func (sstpClient) Connect(ctx context.Context, server Server, out io.Writer) error {
	for _, bin := range []string{"sstpc", "pppd"} {
		if _, err := osexec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found: install sstp-client and ppp (e.g. apt install sstp-client ppp)", bin)
		}
	}

	ip := server.IPAddr
	if ip == "" {
		return fmt.Errorf("cannot connect with sstp to %s: relay has no IP address", server.HostName)
	}

	dir, err := os.MkdirTemp("", "vpngate-sstp-*")
	if err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	debug := os.Getenv("VPNGATE_DEBUG") != ""
	script := filepath.Join(dir, "sstp-up.sh")
	if err := os.WriteFile(script, []byte(sstpScript(ip, debug)), 0o755); err != nil {
		return fmt.Errorf("write tunnel script: %w", err)
	}

	return exec.RunContext(ctx, "/bin/sh", dir, out, script)
}

// ConnectDetached is unsupported for SSTP: background mode needs the
// openvpn management socket and is therefore openvpn-only.
func (sstpClient) ConnectDetached(_ Server, _, _ string, _ io.Writer, _ *syscall.SysProcAttr) (*osexec.Cmd, error) {
	return nil, fmt.Errorf("the sstp protocol does not support background (daemon) mode, use the openvpn protocol instead")
}

// sstpArgs returns the sstpc command line for a vpngate relay. Kept as a
// pure function so the exact arguments can be unit-tested. sstp-client
// v1.0.x removed the --noproxy/--defaultroute options: routing and proxy
// control are passed as pppd options after the "--" separator, and proxy
// environment variables are neutralized by the wrapper script.
func sstpArgs(ip string, debug bool) []string {
	args := []string{
		"--cert-warn",
		"--user", vpngateSSTPUser,
		"--password", vpngateSSTPPassword,
		"--save-server-route",
	}
	if debug {
		args = append(args, "--log-level", "3", "--log-stderr")
	}
	return append(args, ip, "--", "defaultroute", "replacedefaultroute")
}

// sstpScript returns the wrapper script that drives the tunnel. It starts
// sstpc in the background, then prints SSTP_UP once a fresh ppp interface
// carrying the default route appears. The script stays alive as long as
// sstpc and the interface exist, and kills sstpc on exit so no daemon is
// left behind.
func sstpScript(ip string, debug bool) string {
	return `#!/bin/sh
set -e
iface=""
before=$(ls /sys/class/net 2>/dev/null)

cleanup() {
	[ -n "$sstpc_pid" ] && kill "$sstpc_pid" 2>/dev/null || true
}
trap 'exit 0' TERM INT
trap cleanup EXIT

env -u http_proxy -u https_proxy -u all_proxy sstpc ` + strings.Join(sstpArgs(ip, debug), " ") + ` &
	sstpc_pid=$!

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do
	for n in $(ls /sys/class/net 2>/dev/null); do
		case " $before " in *" $n "*) ;;
			*)
				iface=$n
				break 2
			;;
		esac
	done
	sleep 1
done

if [ -z "$iface" ]; then
	echo "SSTP_FAIL: session never came up (relay down, wrong credentials or TLS issue)" >&2
	exit 1
fi

echo "SSTP_UP iface=$iface"

while kill -0 "$sstpc_pid" 2>/dev/null && [ -e /sys/class/net/$iface ]; do
	sleep 2
done
exit 0
`
}