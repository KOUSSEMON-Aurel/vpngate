package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
		mode:       ModeBrowse,
		round:      3,
		cursorHost: "a",
		width:      100,
		height:     24,
		quitting:   false,
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

func TestViewWithGeoShowsMapAndLocation(t *testing.T) {
	a := testServer("a")
	m := &model{
		servers: []vpn.Server{a},
		results: map[string]vpn.ProbeResult{
			"a": {Status: vpn.ProbeWorking, LatencyMs: 42},
		},
		mode:       ModeBrowse,
		round:      3,
		cursorHost: "a",
		width:      140,
		height:     24,
		geo: geoInfo{
			loaded: true,
			code:   "FR",
			name:   "France",
			city:   "Paris",
		},
	}

	out := m.View()
	for _, want := range []string{"YOU", "●", "France", "Paris", "🇫🇷"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q in:\n%s", want, out)
		}
	}
}

func TestViewWithoutGeoFallsBack(t *testing.T) {
	a := testServer("a")
	m := &model{
		servers: []vpn.Server{a},
		results: map[string]vpn.ProbeResult{
			"a": {Status: vpn.ProbeWorking, LatencyMs: 42},
		},
		mode:       ModeBrowse,
		round:      1,
		cursorHost: "a",
		width:      140,
		height:     24,
	}
	out := m.View()
	if n := strings.Count(out, "locating"); n != 2 {
		t.Errorf("View() should show locating fallback twice (map + geo bar), got %d in:\n%s", n, out)
	}
	// The map column must not pin a marker before geo resolves. Status icons
	// also use "●", so scope the check to the map region (below the "YOU"
	// header, right-hand column).
	lines := strings.Split(out, "\n")
	youIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "YOU") {
			youIdx = i
			break
		}
	}
	if youIdx < 0 {
		t.Fatalf("View() missing YOU header in:\n%s", out)
	}
	for _, l := range lines[youIdx+1 : youIdx+1+11] {
		mapCol := ""
		if len(l) > 30 {
			mapCol = l[len(l)-30:]
		}
		if strings.Contains(mapCol, "●") {
			t.Errorf("map column pinned a marker without geo in line %q", l)
		}
	}
}

func TestViewShowsMapAtNarrowWidth(t *testing.T) {
	a := testServer("a-long-hostname.example")
	m := &model{
		servers: []vpn.Server{a},
		results: map[string]vpn.ProbeResult{
			"a": {Status: vpn.ProbeWorking, LatencyMs: 42},
		},
		mode:       ModeBrowse,
		round:      1,
		cursorHost: "a",
		width:      80,
		height:     24,
		geo: geoInfo{
			loaded: true,
			code:   "FR",
			name:   "France",
			city:   "Paris",
		},
	}
	out := m.View()
	if !strings.Contains(out, "YOU") {
		t.Errorf("View() should show the world map at 80 columns, got:\n%s", out)
	}
	if strings.Contains(out, "SCORE") {
		t.Errorf("View() at 80 cols should drop the SCORE column (compact layout), got:\n%s", out)
	}
}

func TestViewLinesNeverExceedWidth(t *testing.T) {
	a := testServer("a-long-hostname.example.com")
	for _, width := range []int{60, 80, 100, 140, 200} {
		m := &model{
			servers: []vpn.Server{a},
			results: map[string]vpn.ProbeResult{
				"a": {Status: vpn.ProbeWorking, LatencyMs: 1234},
			},
			mode:       ModeBrowse,
			round:      3,
			cursorHost: "a",
			width:      width,
			height:     24,
			geo: geoInfo{
				loaded: true,
				code:   "FR",
				name:   "France",
				city:   "Paris",
			},
		}
		for i, l := range strings.Split(m.View(), "\n") {
			if vw := lipgloss.Width(l); vw > width {
				t.Errorf("width %d: line %d is %d visible columns wide:\n%q", width, i, vw, l)
			}
		}
	}
}

func TestDisplayServersOrderStableWithinRound(t *testing.T) {
	a, b, c, d := testServer("a"), testServer("b"), testServer("c"), testServer("d")
	m := &model{
		servers: []vpn.Server{c, a, d, b},
		results: map[string]vpn.ProbeResult{
			"b": {Status: vpn.ProbeWorking, LatencyMs: 80},
			"c": {Status: vpn.ProbeWorking, LatencyMs: 20},
			"a": {Status: vpn.ProbeAuthFailed},
		},
		round: 7,
	}

	first := m.displayServers()

	// Same round: latency flips and a new result appears, but the row order
	// must stay frozen so the cursor does not jump mid-round.
	m.results["b"] = vpn.ProbeResult{Status: vpn.ProbeWorking, LatencyMs: 10}
	m.results["d"] = vpn.ProbeResult{Status: vpn.ProbeWorking, LatencyMs: 5}

	second := m.displayServers()
	if len(first) != len(second) {
		t.Fatalf("length changed: %v vs %v", first, second)
	}
	for i := range first {
		if first[i].HostName != second[i].HostName {
			t.Fatalf("order changed mid-round at %d: %v vs %v", i, first, second)
		}
	}

	// Advancing the round lets the new order through.
	m.round = 8
	third := m.displayServers()
	if third[0].HostName != "d" {
		t.Fatalf("expected new fastest first after round advance, got %v", third)
	}
}
