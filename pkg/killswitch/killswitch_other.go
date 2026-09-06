//go:build !linux && !darwin && !windows

package killswitch

import (
	"context"

	"github.com/davegallant/vpngate/pkg/vpn"
)

type noopKillSwitch struct{}

// New returns a stub KillSwitch for platforms without native firewall support.
func New() KillSwitch {
	return &noopKillSwitch{}
}

func (k *noopKillSwitch) Name() string {
	return "unsupported"
}

func (k *noopKillSwitch) IsActive() bool {
	return false
}

func (k *noopKillSwitch) Enable(ctx context.Context, s vpn.Server) error {
	return ErrUnsupportedPlatform
}

func (k *noopKillSwitch) OnTunnelUp(ctx context.Context, dev string) error {
	return nil
}

func (k *noopKillSwitch) Disable(ctx context.Context) error {
	return nil
}
