package vpn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestL2TPConfigFiles verifies the generated strongswan / xl2tpd / pppd
// configs for a vpngate relay: the relay IP, the shared VPN Gate
// credentials, and the tunnel routing options.
func TestL2TPConfigFiles(t *testing.T) {
	files, err := l2tpConfigFiles(Server{HostName: "test-relay-1", IPAddr: "203.0.113.7"}, "/tmp/vpngate-test")
	require.NoError(t, err)

	assert.Contains(t, string(files["ipsec.conf"]), "right=203.0.113.7")
	assert.Contains(t, string(files["ipsec.conf"]), "rightprotoport=17/1701")
	assert.Contains(t, string(files["ipsec.conf"]), "forceencaps=yes")
	assert.Contains(t, string(files["ipsec.secrets"]), `: PSK "vpn"`)
	assert.Contains(t, string(files["xl2tpd.conf"]), "lns = 203.0.113.7")
	assert.Contains(t, string(files["xl2tpd.conf"]), "autodial = yes")
	assert.Contains(t, string(files["options"]), "name vpn")
	assert.Contains(t, string(files["options"]), "password vpn")
	assert.Contains(t, string(files["options"]), "defaultroute")
	assert.Contains(t, string(files["options"]), "replacedefaultroute")
}

// TestL2TPConfigFilesNoIP verifies that a relay without an IP address is
// rejected before any config is generated.
func TestL2TPConfigFilesNoIP(t *testing.T) {
	_, err := l2tpConfigFiles(Server{HostName: "broken"}, "/tmp/vpngate-test")
	require.Error(t, err)
}

// TestL2TPScript verifies the wrapper script structure: it brings up the
// SA, dials L2TP, waits for a fresh ppp interface, prints the L2TP_UP
// marker, and tears the daemons down on exit.
func TestL2TPScript(t *testing.T) {
	s := l2tpScript("/tmp/vpngate-test")
	for _, want := range []string{
		"ipsec start --nofork",
		"ipsec up vpngate",
		`xl2tpd -c "$dir/xl2tpd.conf" -D`,
		"L2TP_UP iface=$iface",
		"trap cleanup EXIT",
		"ls /sys/class/net",
		"mkdir -p /var/run/xl2tpd",
		`cp "$dir/ipsec.conf" "$ipsec_cfg"`,
		`cp "$dir/ipsec.secrets" "$ipsec_secrets"`,
		`cp "$dir/ipsec.conf.bak" "$ipsec_cfg"`,
	} {
		assert.Contains(t, s, want)
	}
	// The credentials live in the generated config files (covered by
	// TestL2TPConfigFiles), never in the script itself.
	assert.NotContains(t, s, "password")
}

// TestSSTPArgs verifies the sstpc command line for a vpngate relay:
// shared credentials, routing flags, TLS warning and the pppd routing
// options passed after the "--" separator.
func TestSSTPArgs(t *testing.T) {
	args := sstpArgs("203.0.113.7", false)
	assert.Equal(t, "203.0.113.7", args[6])
	assert.Contains(t, args, "--cert-warn")
	assert.Contains(t, args, "--save-server-route")
	assert.Contains(t, args, "--user")
	assert.Contains(t, args, "vpn")
	assert.Contains(t, args, "--")
	assert.Contains(t, args, "defaultroute")
	assert.Contains(t, args, "replacedefaultroute")
	assert.NotContains(t, args, "--log-level")

	debug := sstpArgs("203.0.113.7", true)
	assert.Contains(t, debug, "--log-level")
	assert.Contains(t, debug, "3")
	assert.Contains(t, debug, "--log-stderr")
}

// TestSSTPScript verifies the wrapper script structure: it starts sstpc,
// waits for a fresh ppp interface, prints the SSTP_UP marker and kills
// sstpc on exit.
func TestSSTPScript(t *testing.T) {
	s := sstpScript("203.0.113.7", false)
	assert.Contains(t, s, "env -u http_proxy -u https_proxy -u all_proxy sstpc")
	assert.Contains(t, s, "SSTP_UP iface=$iface")
	assert.Contains(t, s, "trap cleanup EXIT")

	debug := sstpScript("203.0.113.7", true)
	assert.Contains(t, debug, "--log-level 3 --log-stderr")
	assert.True(t, strings.HasPrefix(s, "#!/bin/sh"))
}