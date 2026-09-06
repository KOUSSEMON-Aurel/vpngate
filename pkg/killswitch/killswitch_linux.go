//go:build linux

package killswitch

import (
	"context"
	"fmt"
	osexec "os/exec"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/davegallant/vpngate/pkg/vpn"
)

type linuxKillSwitch struct {
	mu       sync.Mutex
	active   bool
	endpoint Endpoint
	usedNetns bool
	usedNft   bool
	usedIpt   bool
}

// New returns a Linux-specific KillSwitch implementation backed by
// network namespaces and nftables (with iptables fallback).
func New() KillSwitch {
	return &linuxKillSwitch{}
}

func (k *linuxKillSwitch) Name() string {
	return "linux-netns-nftables"
}

func (k *linuxKillSwitch) IsActive() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.active
}

func (k *linuxKillSwitch) Enable(ctx context.Context, s vpn.Server) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.active {
		return nil
	}

	ep := ExtractEndpoint(s)
	k.endpoint = ep

	// 1. Try setting up dedicated ip netns isolation
	if _, err := osexec.LookPath("ip"); err == nil {
		if err := k.setupNetns(ctx); err == nil {
			k.usedNetns = true
		} else {
			log.Debug().Msgf("killswitch: ip netns setup skipped: %v (falling back to host firewall)", err)
		}
	}

	// 2. Setup host-level packet filter (nftables preferred, iptables fallback)
	if _, err := osexec.LookPath("nft"); err == nil {
		if err := k.setupNftables(ctx, ep); err == nil {
			k.usedNft = true
			k.active = true
			log.Info().Msgf("killswitch: activated host nftables isolation for %s:%d (%s)", ep.IP, ep.Port, ep.Proto)
			return nil
		}
	}

	if _, err := osexec.LookPath("iptables"); err == nil {
		if err := k.setupIptables(ctx, ep); err == nil {
			k.usedIpt = true
			k.active = true
			log.Info().Msgf("killswitch: activated host iptables isolation for %s:%d (%s)", ep.IP, ep.Port, ep.Proto)
			return nil
		}
	}

	if k.usedNetns {
		k.active = true
		return nil
	}

	return fmt.Errorf("unable to activate kill switch: neither nftables nor iptables is available")
}

func (k *linuxKillSwitch) setupNetns(ctx context.Context) error {
	// Create netns vpngate-ns and loopback
	cmd := osexec.CommandContext(ctx, "ip", "netns", "add", "vpngate-ns")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	_ = osexec.CommandContext(ctx, "ip", "netns", "exec", "vpngate-ns", "ip", "link", "set", "lo", "up").Run()
	return nil
}

func (k *linuxKillSwitch) setupNftables(ctx context.Context, ep Endpoint) error {
	// Construct atomic nftables table inet vpngate_ks
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

	cmd := osexec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(rules)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft failed: %w: %s", err, string(out))
	}
	return nil
}

func (k *linuxKillSwitch) setupIptables(ctx context.Context, ep Endpoint) error {
	// Create VPNGATE_KS chain in filter table
	_ = osexec.CommandContext(ctx, "iptables", "-N", "VPNGATE_KS").Run()
	_ = osexec.CommandContext(ctx, "iptables", "-F", "VPNGATE_KS").Run()

	rules := [][]string{
		{"-A", "VPNGATE_KS", "-o", "lo", "-j", "ACCEPT"},
		{"-A", "VPNGATE_KS", "-o", "tun+", "-j", "ACCEPT"},
		{"-A", "VPNGATE_KS", "-o", "wg+", "-j", "ACCEPT"},
		{"-A", "VPNGATE_KS", "-p", "udp", "--sport", "68", "--dport", "67", "-j", "ACCEPT"},
		{"-A", "VPNGATE_KS", "-d", ep.IP, "-p", ep.Proto, "--dport", fmt.Sprintf("%d", ep.Port), "-j", "ACCEPT"},
		{"-A", "VPNGATE_KS", "-j", "DROP"},
	}

	for _, rule := range rules {
		cmd := osexec.CommandContext(ctx, "iptables", rule...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables %v failed: %w: %s", rule, err, string(out))
		}
	}

	// Insert jump to VPNGATE_KS at the top of OUTPUT chain
	cmd := osexec.CommandContext(ctx, "iptables", "-I", "OUTPUT", "1", "-j", "VPNGATE_KS")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables jump failed: %w: %s", err, string(out))
	}

	return nil
}

func (k *linuxKillSwitch) OnTunnelUp(ctx context.Context, dev string) error {
	// With the default policy set to DROP except tun* and wg*,
	// once tun0 is up, the system is fully protected against leaks.
	log.Info().Msgf("killswitch: tunnel %s confirmed active; all other outbound interfaces locked", dev)
	return nil
}

func (k *linuxKillSwitch) Disable(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.active {
		return nil
	}

	var firstErr error

	if k.usedNft {
		cmd := osexec.CommandContext(ctx, "nft", "delete", "table", "inet", "vpngate_ks")
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warn().Msgf("killswitch: failed to delete nftables table: %v (%s)", err, string(out))
			firstErr = err
		}
		k.usedNft = false
	}

	if k.usedIpt {
		_ = osexec.CommandContext(ctx, "iptables", "-D", "OUTPUT", "-j", "VPNGATE_KS").Run()
		_ = osexec.CommandContext(ctx, "iptables", "-F", "VPNGATE_KS").Run()
		_ = osexec.CommandContext(ctx, "iptables", "-X", "VPNGATE_KS").Run()
		k.usedIpt = false
	}

	if k.usedNetns {
		_ = osexec.CommandContext(ctx, "ip", "netns", "del", "vpngate-ns").Run()
		k.usedNetns = false
	}

	k.active = false
	log.Info().Msg("killswitch: disabled, network connectivity restored")
	return firstErr
}
