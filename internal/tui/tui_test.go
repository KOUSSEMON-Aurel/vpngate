package tui

import (
	"strings"
	"testing"

	"github.com/davegallant/vpngate/pkg/vpn"
)

func testServer(name string) vpn.Server {
	return vpn.Server{
		HostName:     name,
		CountryLong:  "Japan",
		CountryShort: "JP",
		Score:        10,
	}
}

func TestStatusIcon(t *testing.T) {
	cases := map[vpn.ProbeStatus]string{
		vpn.ProbeWorking:     "●",
		vpn.ProbeAuthFailed:  "◐",
		vpn.ProbeChecking:    "…",
		vpn.ProbeUnreachable: "○",
		vpn.ProbeTimeout:     "◌",
	}
	for status, want := range cases {
		if got := statusIcon(status); got != want {
			t.Errorf("statusIcon(%v) = %q, want %q", status, got, want)
		}
	}
}

func TestCountryFlag(t *testing.T) {
	if got := countryFlag("jp"); got != "🇯🇵" {
		t.Errorf("countryFlag(jp) = %q", got)
	}
	if got := countryFlag("fr"); got != "🇫🇷" {
		t.Errorf("countryFlag(fr) = %q", got)
	}
	if got := countryFlag(""); got != "" {
		t.Errorf("countryFlag('') = %q, want empty", got)
	}
	if got := countryFlag("USA"); got != "" {
		t.Errorf("countryFlag(USA) = %q, want empty", got)
	}
}

func TestDisplayServersSortsWorkingFirstByLatency(t *testing.T) {
	a := testServer("a")
	b := testServer("b")
	c := testServer("c")
	d := testServer("d")

	m := &model{
		servers: []vpn.Server{c, a, d, b},
		results: map[string]vpn.ProbeResult{
			"b": {Status: vpn.ProbeWorking, LatencyMs: 80},
			"c": {Status: vpn.ProbeWorking, LatencyMs: 20},
			"a": {Status: vpn.ProbeAuthFailed},
			// d has no result (unknown)
		},
		mode:        ModeSelect,
		workingOnly: true,
	}

	got := m.displayServers()
	if len(got) != 2 {
		t.Fatalf("expected 2 working servers, got %d: %v", len(got), got)
	}
	if got[0].HostName != "c" || got[1].HostName != "b" {
		t.Fatalf("expected working sorted by latency [c b], got %v", got)
	}
}

func TestDisplayServersFallbackOrder(t *testing.T) {
	a := testServer("a")
	b := testServer("b")
	c := testServer("c")

	m := &model{
		servers: []vpn.Server{b, a, c},
		results: map[string]vpn.ProbeResult{
			"b": {Status: vpn.ProbeWorking, LatencyMs: 10},
			"c": {Status: vpn.ProbeAuthFailed},
		},
		workingOnly: false,
	}

	got := m.displayServers()
	if got[0].HostName != "b" {
		t.Fatalf("expected working first, got %v", got)
	}
	if got[1].HostName != "c" {
		t.Fatalf("expected auth_failed second, got %v", got)
	}
	if got[2].HostName != "a" {
		t.Fatalf("expected unknown last, got %v", got)
	}
}

func TestViewRendersStatusAndFooter(t *testing.T) {
	a := testServer("a")
	m := &model{
		servers: []vpn.Server{a},
		results: map[string]vpn.ProbeResult{
			"a": {Status: vpn.ProbeWorking, LatencyMs: 42},
		},
		mode:     ModeBrowse,
		round:    3,
		cursor:   0,
		width:    100,
		height:   24,
		quitting: false,
	}

	out := m.View()
	for _, want := range []string{"a", "Japan", "working", "42ms", "round 3", "[q] quit", "1 working"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q in:\n%s", want, out)
		}
	}
}

func TestViewHiddenWhenQuitting(t *testing.T) {
	m := &model{quitting: true}
	if out := m.View(); out != "" {
		t.Errorf("expected empty view when quitting, got %q", out)
	}
}
