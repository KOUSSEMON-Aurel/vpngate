package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/davegallant/vpngate/pkg/vpn"
)

func TestTitleBarBadgeWidth(t *testing.T) {
	for _, priv := range []privState{privRoot, privCapNetAdmin, privNone, privUnknown} {
		m := &model{
			width:   60,
			height:  24,
			mode:    ModeSelect,
			priv:    priv,
			servers: []vpn.Server{testServer("a")},
			results: map[string]vpn.ProbeResult{"a": {Status: vpn.ProbeWorking, LatencyMs: 42}},
		}
		for _, w := range []int{30, 40, 60, 80} {
			m.width = w
			line := m.titleBar(w)
			if vw := lipgloss.Width(line); vw > w {
				t.Errorf("priv %v width %d: badge renders %d columns: %q", priv, w, vw, line)
			}
			badge := m.privBadge()
			if badge != "" && !strings.Contains(line, badge) {
				t.Errorf("priv %v width %d: badge %q lost from line %q", priv, w, badge, line)
			}
		}
	}
}

func TestPrivBadgeContents(t *testing.T) {
	m := &model{priv: privRoot}
	if got := m.privBadge(); !strings.Contains(got, "sudo") {
		t.Errorf("privRoot badge = %q, want sudo", got)
	}
	m.priv = privNone
	if got := m.privBadge(); !strings.Contains(got, "no sudo") {
		t.Errorf("privNone badge = %q, want 'no sudo'", got)
	}
	m.priv = privCapNetAdmin
	if got := m.privBadge(); !strings.Contains(got, "cap") {
		t.Errorf("privCapNetAdmin badge = %q, want cap", got)
	}
	m.priv = privUnknown
	if got := m.privBadge(); got != "" {
		t.Errorf("privUnknown badge = %q, want empty", got)
	}
}

func TestDetectPrivilegeCoherentWithEuid(t *testing.T) {
	got := detectPrivilege()
	if os.Geteuid() == 0 {
		if got != privRoot {
			t.Errorf("euid==0 but detectPrivilege = %v, want privRoot", got)
		}
		return
	}
	if got == privRoot {
		t.Errorf("euid!=0 but detectPrivilege = privRoot")
	}
	t.Logf("detectPrivilege (non-root) = %v", got)
}

func TestOpenvpnCapHelperRuns(t *testing.T) {
	// Must not panic on this system regardless of setcap state.
	_ = processHasCapNetAdmin()
	_ = openvpnHasCapNetAdmin()
}
