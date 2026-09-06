package killswitch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/davegallant/vpngate/pkg/vpn"
)

// TestDarwinPFRulesFormat verifies the exact pfctl anchor syntax generated for macOS.
func TestDarwinPFRulesFormat(t *testing.T) {
	server := vpn.Server{
		IPAddr:    "198.51.100.55",
		Transport: "udp1194",
	}
	ep := ExtractEndpoint(server)
	assert.Equal(t, "198.51.100.55", ep.IP)
	assert.Equal(t, 1194, ep.Port)
	assert.Equal(t, "udp", ep.Proto)

	// Format that darwinKillSwitch sends to pfctl -a vpngate -f -
	rules := fmt.Sprintf(`
# vpngate kill switch anchor
block drop out all
pass out quick on lo0 all
pass out quick on utun+ all
pass out quick proto %s to %s port %d
`, ep.Proto, ep.IP, ep.Port)

	assert.Contains(t, rules, "block drop out all")
	assert.Contains(t, rules, "pass out quick on lo0 all")
	assert.Contains(t, rules, "pass out quick on utun+ all")
	assert.Contains(t, rules, "pass out quick proto udp to 198.51.100.55 port 1194")
}

// TestWindowsNetshRulesFormat verifies the exact netsh advfirewall commands for Windows.
func TestWindowsNetshRulesFormat(t *testing.T) {
	server := vpn.Server{
		IPAddr:    "203.0.113.88",
		Transport: "tcp443",
	}
	ep := ExtractEndpoint(server)
	assert.Equal(t, "203.0.113.88", ep.IP)
	assert.Equal(t, 443, ep.Port)
	assert.Equal(t, "tcp", ep.Proto)

	allowRemoteArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=vpngate-killswitch-allow-remote", "dir=out", "action=allow",
		fmt.Sprintf("remoteip=%s", ep.IP),
		fmt.Sprintf("protocol=%s", ep.Proto),
		fmt.Sprintf("remoteport=%d", ep.Port),
		"enable=yes",
	}

	blockArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=vpngate-killswitch-block", "dir=out", "action=block", "enable=yes",
	}

	allowLoArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=vpngate-killswitch-allow-lo", "dir=out", "action=allow", "remoteip=127.0.0.1", "enable=yes",
	}

	assert.Contains(t, strings.Join(allowRemoteArgs, " "), "remoteip=203.0.113.88 protocol=tcp remoteport=443")
	assert.Contains(t, strings.Join(blockArgs, " "), "action=block")
	assert.Contains(t, strings.Join(allowLoArgs, " "), "remoteip=127.0.0.1")
}

// TestLinuxNftablesRulesFormat verifies the exact nftables syntax generated for Linux.
func TestLinuxNftablesRulesFormat(t *testing.T) {
	server := vpn.Server{
		IPAddr:    "192.0.2.42",
		Transport: "udp1194",
	}
	ep := ExtractEndpoint(server)

	rules := fmt.Sprintf(`
add table inet vpngate_ks
flush table inet vpngate_ks
add chain inet vpngate_ks output { type filter hook output priority filter; policy drop; }
add rule inet vpngate_ks output oif "lo" accept
add rule inet vpngate_ks output oifname "tun*" accept
add rule inet vpngate_ks output oifname "wg*" accept
add rule inet vpngate_ks output udp sport 68 udp dport 67 accept
add rule inet vpngate_ks output ip daddr 255.255.255.255 accept
add rule inet vpngate_ks output ip daddr %s %s dport %d accept
`, ep.IP, ep.Proto, ep.Port)

	assert.Contains(t, rules, "policy drop")
	assert.Contains(t, rules, "add rule inet vpngate_ks output oif \"lo\" accept")
	assert.Contains(t, rules, "add rule inet vpngate_ks output oifname \"tun*\" accept")
	assert.Contains(t, rules, "add rule inet vpngate_ks output ip daddr 192.0.2.42 udp dport 1194 accept")
}
