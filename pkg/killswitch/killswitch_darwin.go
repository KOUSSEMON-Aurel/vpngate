//go:build darwin

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

type darwinKillSwitch struct {
	mu       sync.Mutex
	active   bool
	endpoint Endpoint
}

// New returns a macOS-specific KillSwitch implementation using pfctl.
func New() KillSwitch {
	return &darwinKillSwitch{}
}

func (k *darwinKillSwitch) Name() string {
	return "macos-pf"
}

func (k *darwinKillSwitch) IsActive() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.active
}

func (k *darwinKillSwitch) Enable(ctx context.Context, s vpn.Server) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.active {
		return nil
	}

	ep := ExtractEndpoint(s)
	k.endpoint = ep

	rules := fmt.Sprintf(`
# vpngate kill switch anchor
block drop out all
pass out quick on lo0 all
pass out quick on utun+ all
pass out quick proto %s to %s port %d
`, ep.Proto, ep.IP, ep.Port)

	// Load pf anchor vpngate
	cmd := osexec.CommandContext(ctx, "pfctl", "-a", "vpngate", "-f", "-")
	cmd.Stdin = strings.NewReader(rules)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pfctl anchor load failed: %w (%s)", err, string(out))
	}

	// Enable pf if not already running
	_ = osexec.CommandContext(ctx, "pfctl", "-e").Run()

	k.active = true
	log.Info().Msgf("killswitch: activated macOS pf anchor for %s:%d (%s)", ep.IP, ep.Port, ep.Proto)
	return nil
}

func (k *darwinKillSwitch) OnTunnelUp(ctx context.Context, dev string) error {
	log.Info().Msgf("killswitch: macOS tunnel %s up; traffic restricted to utun", dev)
	return nil
}

func (k *darwinKillSwitch) Disable(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.active {
		return nil
	}

	cmd := osexec.CommandContext(ctx, "pfctl", "-a", "vpngate", "-F", "all")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warn().Msgf("killswitch: failed to flush pf anchor: %v (%s)", err, string(out))
		return err
	}

	k.active = false
	log.Info().Msg("killswitch: macOS pf anchor cleared, connectivity restored")
	return nil
}
