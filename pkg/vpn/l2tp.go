package vpn

import (
	"context"
	"fmt"
	"io"
	osexec "os/exec"
	"os"
	"path/filepath"
	"syscall"

	"github.com/davegallant/vpngate/pkg/exec"
)

// vpngateL2TPUser, vpngateL2TPPassword and vpngateL2TPPSK are the shared
// credentials VPN Gate publishes for every L2TP/IPsec relay. They are
// public and fixed for the whole pool, so unlike OpenVPN (one config per
// relay) no per-server credential is needed.
const (
	vpngateL2TPUser     = "vpn"
	vpngateL2TPPassword = "vpn"
	vpngateL2TPPSK      = "vpn"
)

// l2tpClient connects to a vpngate relay through an L2TP/IPsec tunnel
// built from strongswan (IPsec) + xl2tpd (L2TP) + pppd. The three daemons
// run inside a single generated wrapper script so the whole tunnel is one
// child process group that can be torn down together.
type l2tpClient struct{}

func (l2tpClient) Connect(ctx context.Context, server Server, out io.Writer) error {
	for _, bin := range []string{"ipsec", "xl2tpd", "pppd"} {
		if _, err := osexec.LookPath(bin); err != nil {
			return fmt.Errorf("%s not found: install strongswan-starter, xl2tpd and ppp (e.g. apt install strongswan-starter xl2tpd ppp)", bin)
		}
	}

	dir, err := os.MkdirTemp("", "vpngate-l2tp-*")
	if err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	files, err := l2tpConfigFiles(server, dir)
	if err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	script := filepath.Join(dir, "l2tp-up.sh")
	if err := os.WriteFile(script, []byte(l2tpScript(dir)), 0o755); err != nil {
		return fmt.Errorf("write tunnel script: %w", err)
	}

	return exec.RunContext(ctx, "/bin/sh", dir, out, script)
}

// ConnectDetached is unsupported for L2TP/IPsec: background mode needs the
// openvpn management socket and is therefore openvpn-only.
func (l2tpClient) ConnectDetached(_ Server, _, _ string, _ io.Writer, _ *syscall.SysProcAttr) (*osexec.Cmd, error) {
	return nil, fmt.Errorf("the l2tp/ipsec protocol does not support background (daemon) mode, use the openvpn protocol instead")
}

// l2tpConfigFiles returns the generated strongswan / xl2tpd / pppd configs
// for a vpngate relay, keyed by file name. Kept as a pure function so the
// exact output can be unit-tested.
func l2tpConfigFiles(server Server, dir string) (map[string][]byte, error) {
	ip := server.IPAddr
	if ip == "" {
		return nil, fmt.Errorf("cannot build l2tp/ipsec config for %s: relay has no IP address", server.HostName)
	}

	return map[string][]byte{
		"ipsec.conf": []byte(`conn %default
	keyexchange=ikev1
	authby=secret
	ike=aes128-sha1-modp1024
	esp=aes128-sha1-modp1024
	forceencaps=yes
	keyingtries=1

conn vpngate
	type=transport
	left=%defaultroute
	leftprotoport=17/1701
	right=` + ip + `
	rightprotoport=17/1701
	auto=add
`),
		"ipsec.secrets": []byte(`: PSK "` + vpngateL2TPPSK + `"
`),
		"xl2tpd.conf": []byte(`[global]
port = 1701

[lac vpngate]
lns = ` + ip + `
autodial = yes
redial = no
require pap = yes
require chap = yes
ppp debug = yes
pppoptfile = ` + filepath.Join(dir, "options") + `
`),
		"options": []byte(`name ` + vpngateL2TPUser + `
password ` + vpngateL2TPPassword + `
require-mschap-v2
noauth
noipdefault
defaultroute
replacedefaultroute
`),
	}, nil
}

// l2tpScript returns the wrapper script that drives the tunnel. strongswan
// starter only reads /etc/ipsec.conf, so the generated config is installed
// there for the duration of the tunnel (with the originals backed up and
// restored on exit). The script then brings the vpngate SA up, dials the
// L2TP session through xl2tpd (autodial), and prints L2TP_UP once a fresh
// ppp interface appears. It stays alive as long as the SA, the daemon and
// the interface exist, and tears everything down on exit so no daemon or
// config change is left behind.
func l2tpScript(dir string) string {
	return `#!/bin/sh
set -e
dir=` + dir + `
iface=""
before=$(ls /sys/class/net 2>/dev/null | tr "\n" " ")
ipsec_cfg=/etc/ipsec.conf
ipsec_secrets=/etc/ipsec.secrets
backed_up=""

cleanup() {
	if [ -n "$backed_up" ]; then
		cp "$dir/ipsec.conf.bak" "$ipsec_cfg" 2>/dev/null || true
		cp "$dir/ipsec.secrets.bak" "$ipsec_secrets" 2>/dev/null || true
	fi
	ipsec down vpngate 2>/dev/null || true
	ipsec stop 2>/dev/null || true
	[ -n "$xl_pid" ] && kill "$xl_pid" 2>/dev/null || true
}
trap 'exit 0' TERM INT
trap cleanup EXIT

if [ -f "$ipsec_cfg" ]; then
	cp "$ipsec_cfg" "$dir/ipsec.conf.bak"
	backed_up=1
	if [ -f "$ipsec_secrets" ]; then
		cp "$ipsec_secrets" "$dir/ipsec.secrets.bak"
	fi
fi
cp "$dir/ipsec.conf" "$ipsec_cfg"
cp "$dir/ipsec.secrets" "$ipsec_secrets"

ipsec start --nofork &
ipsec_pid=$!
sleep 2
if ! ipsec up vpngate; then
	echo "L2TP_FAIL: IPsec SA negotiation failed (wrong PSK, charon already running, or relay down)" >&2
	exit 1
fi

# xl2tpd refuses to dial without its control directory; the Debian init
# script creates it, running xl2tpd directly does not.
mkdir -p /var/run/xl2tpd
xl2tpd -c "$dir/xl2tpd.conf" -D &
xl_pid=$!

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
	echo "L2TP_FAIL: L2TP session never came up (relay down or wrong credentials)" >&2
	exit 1
fi

echo "L2TP_UP iface=$iface"

while kill -0 "$ipsec_pid" 2>/dev/null && kill -0 "$xl_pid" 2>/dev/null && [ -e /sys/class/net/$iface ]; do
	sleep 2
done
exit 0
`
}