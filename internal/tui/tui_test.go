package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/davegallant/vpngate/pkg/vpn"
	"github.com/muesli/termenv"
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

func TestLandAt(t *testing.T) {
	land := func(lat, lon float64) bool { return landAt(lat, lon) }
	cases := []struct {
		lat, lon float64
		want     bool
	}{
		{51.5, -0.1, true},   // London
		{48.8, 2.3, true},    // Paris
		{35.6, 139.7, true},  // Tokyo
		{-33.8, 151.2, true}, // Sydney
		{0, 0, false},        // Gulf of Guinea (ocean)
		{0, -130, false},     // mid-Pacific
		{-14.5, -48.2, true}, // central Brazil
		{30, 20, true},       // Sahara
		{90, 0, false},       // North Pole (Arctic ocean)
		{-90, 0, true},       // South Pole (Antarctica)
	}
	for _, c := range cases {
		if got := land(c.lat, c.lon); got != c.want {
			t.Errorf("landAt(%v, %v) = %v, want %v", c.lat, c.lon, got, c.want)
		}
	}
}

func TestGlobeRendersWithinColumnAndPinsMarker(t *testing.T) {
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
		globeRot:   0,
		geo: geoInfo{
			loaded: true,
			code:   "JP",
			name:   "Japan",
			city:   "Tokyo",
		},
	}

	gv := m.globeView()
	if gv == nil {
		t.Fatal("globeView() returned nil at 140x24")
	}
	// 14 rows -> radius 7 -> 2*7+3 = 17 columns.
	if gv.width != 17 {
		t.Errorf("globeView width = %d, want 17", gv.width)
	}
	if len(gv.lines) != 17 {
		t.Errorf("globeView has %d lines, want 17", len(gv.lines))
	}

	pinned := false
	for _, l := range gv.lines {
		if vw := lipgloss.Width(l); vw > gv.width {
			t.Errorf("globe line exceeds column width %d: %q (%d)", gv.width, l, vw)
		}
		if strings.Contains(l, "●") {
			pinned = true
		}
	}
	if !pinned {
		t.Errorf("globeView did not pin the Japan marker in:\n%s", strings.Join(gv.lines, "\n"))
	}
}

func TestGlobePinsExactCoordsAndVpnMarker(t *testing.T) {
	a := testServer("a")
	base := func() *model {
		return &model{
			servers: []vpn.Server{a},
			results: map[string]vpn.ProbeResult{
				"a": {Status: vpn.ProbeWorking, LatencyMs: 42},
			},
			mode:       ModeBrowse,
			round:      1,
			cursorHost: "a",
			width:      140,
			height:     24,
			globeRot:   0,
		}
	}

	// Exact API coordinates win over the country table fallback.
	m := base()
	m.geo = geoInfo{loaded: true, code: "BJ", name: "Benin", city: "Abomey-Calavi", lat: 6.4485, lon: 2.3557}
	out := strings.Join(m.globeView().lines, "\n")
	if !strings.Contains(out, "YOU") || !strings.Contains(out, "●") {
		t.Errorf("globe with exact coords missing YOU marker in:\n%s", out)
	}

	// A VPN exit is labelled VPN and pinned with the VPN colour. Force a
	// colour profile so lipgloss emits ANSI codes and the two markers
	// actually differ (without a profile every style renders plain).
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)
	m = base()
	m.geo = geoInfo{loaded: true, code: "NL", name: "Netherlands", city: "Amsterdam", lat: 52.37, lon: 4.89, vpn: true}
	gv := m.globeView()
	out = strings.Join(gv.lines, "\n")
	if !strings.Contains(out, "VPN") {
		t.Errorf("globe with vpn geo missing VPN label in:\n%s", out)
	}
	vpnDot := styleMarkerVpn.Render("●")
	youDot := styleMarker.Render("●")
	if !strings.Contains(out, vpnDot) {
		t.Errorf("globe with vpn geo missing VPN-coloured marker (have YOU dot %q) in:\n%s", youDot, out)
	}
	if strings.Contains(out, youDot) {
		t.Errorf("globe with vpn geo must not use the plain YOU marker in:\n%s", out)
	}

	// The cache is reused while nothing changed and rebuilt when dirtied.
	m = base()
	m.geo = geoInfo{loaded: true, code: "JP", name: "Japan", lat: 36.2, lon: 138.3}
	first := m.globeView()
	if second := m.globeView(); second != first {
		t.Error("globeView() rebuilt while clean; cache must be reused")
	}
	m.globeDirty = true
	if second := m.globeView(); second == first {
		t.Error("globeView() reused cache while dirty")
	}
}

func TestGlobeHiddenWhenShort(t *testing.T) {
	a := testServer("a")
	m := &model{
		servers: []vpn.Server{a},
		results: map[string]vpn.ProbeResult{
			"a": {Status: vpn.ProbeWorking, LatencyMs: 42},
		},
		width:  140,
		height: 12, // visibleRows = 6 < 7 -> no globe
	}
	if gv := m.globeView(); gv != nil {
		t.Errorf("globeView() should be nil at height 12, got width %d", gv.width)
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

// TestViewFitsHeightEverySize asserts the responsive invariant: at any real
// terminal height the rendered frame is never taller than the terminal, the
// title bar always stays on line 0, and at least one list row is visible. As
// the terminal shrinks the least important chrome (geo bar, footer, detail,
// blank, header, separator, status) is hidden rather than overflowed.
func TestViewFitsHeightEverySize(t *testing.T) {
	servers := []vpn.Server{}
	for i := 0; i < 30; i++ {
		servers = append(servers, testServer(string(rune('a'+i))))
	}
	results := map[string]vpn.ProbeResult{}
	for _, s := range servers {
		results[s.HostName] = vpn.ProbeResult{Status: vpn.ProbeWorking, LatencyMs: 42}
	}

	for height := 3; height <= 40; height++ {
		for _, width := range []int{30, 50, 80, 140} {
			m := &model{
				servers:    servers,
				results:    results,
				mode:       ModeBrowse,
				round:      1,
				cursorHost: "a",
				width:      width,
				height:     height,
				globeRot:   0,
				geo: geoInfo{
					loaded: true,
					code:   "JP",
					name:   "Japan",
					city:   "Tokyo",
				},
			}
			fl := m.frameLayout()
			if got := fl.rows; got < 1 {
				t.Errorf("%dx%d: rows %d < 1", width, height, got)
			}
			frameH := frameHeight(fl)
			if frameH > height {
				t.Errorf("%dx%d: frame height %d > terminal %d (\n%+v)", width, height, frameH, height, fl)
			}

			lines := strings.Split(strings.TrimRight(m.View(), "\n"), "\n")
			if len(lines) > height {
				t.Errorf("%dx%d: View() emitted %d lines, terminal is %d", width, height, len(lines), height)
			}
			if got := lines[0]; !strings.Contains(got, "vpngate") {
				t.Errorf("%dx%d: title lost from line 0: %q", width, height, got)
			}
		}
	}
}
