//go:build windows

package killswitch

import (
	"context"
	"fmt"
	osexec "os/exec"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/davegallant/vpngate/pkg/vpn"
)

type windowsKillSwitch struct {
	mu       sync.Mutex
	active   bool
	endpoint Endpoint
}

// New returns a Windows-specific KillSwitch implementation using netsh advfirewall.
func New() KillSwitch {
	return &windowsKillSwitch{}
}

func (k *windowsKillSwitch) Name() string {
	return "windows-netsh"
}

func (k *windowsKillSwitch) IsActive() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.active
}

func (k *windowsKillSwitch) Enable(ctx context.Context, s vpn.Server) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.active {
		return nil
	}

	ep := ExtractEndpoint(s)
	k.endpoint = ep

	// Allow loopback
	_ = osexec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
		"name=vpngate-killswitch-allow-lo", "dir=out", "action=allow", "remoteip=127.0.0.1", "enable=yes").Run()

	// Allow remote VPN server handshake
	cmdAllowRemote := osexec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
		"name=vpngate-killswitch-allow-remote", "dir=out", "action=allow",
		fmt.Sprintf("remoteip=%s", ep.IP),
		fmt.Sprintf("protocol=%s", ep.Proto),
		fmt.Sprintf("remoteport=%d", ep.Port),
		"enable=yes")
	if out, err := cmdAllowRemote.CombinedOutput(); err != nil {
		return fmt.Errorf("netsh allow remote failed: %w (%s)", err, string(out))
	}

	// Block all other outbound traffic
	cmdBlock := osexec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
		"name=vpngate-killswitch-block", "dir=out", "action=block", "enable=yes")
	if out, err := cmdBlock.CombinedOutput(); err != nil {
		_ = k.cleanupRules(ctx)
		return fmt.Errorf("netsh block failed: %w (%s)", err, string(out))
	}

	k.active = true
	log.Info().Msgf("killswitch: activated Windows firewall isolation for %s:%d (%s)", ep.IP, ep.Port, ep.Proto)
	return nil
}

func (k *windowsKillSwitch) OnTunnelUp(ctx context.Context, dev string) error {
	log.Info().Msgf("killswitch: Windows tunnel %s active", dev)
	return nil
}

func (k *windowsKillSwitch) Disable(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.active {
		return nil
	}

	err := k.cleanupRules(ctx)
	k.active = false
	log.Info().Msg("killswitch: Windows firewall rules removed, connectivity restored")
	return err
}

func (k *windowsKillSwitch) cleanupRules(ctx context.Context) error {
	var firstErr error
	rules := []string{
		"vpngate-killswitch-block",
		"vpngate-killswitch-allow-remote",
		"vpngate-killswitch-allow-lo",
	}
	for _, rule := range rules {
		cmd := osexec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", rule))
		if err := cmd.Run(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
