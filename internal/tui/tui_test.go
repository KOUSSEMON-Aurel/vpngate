package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"
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
	// 14 rows -> radius 8 -> 2*8+3 = 19 columns.
	if gv.width != 19 {
		t.Errorf("globeView width = %d, want 19", gv.width)
	}
	if len(gv.lines) != 19 {
		t.Errorf("globeView has %d lines, want 19", len(gv.lines))
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

// The globe and the bottom location bar must follow the tunnel exit once a
// connection is up, not keep pinning the location resolved at startup.
func TestViewShowsTunnelExitWhileConnected(t *testing.T) {
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
		geo: geoInfo{
			loaded: true,
			code:   "BJ",
			name:   "Benin",
			city:   "Abomey-Calavi",
		},
		connect: &connectState{
			server:    a, // JP in the server table
			connected: true,
			exitIP:    "185.250.249.92",
			exitCC:    "DE",
			exitGeo:   "Germany Neu-Anspach",
		},
	}
	out := m.View()
	for _, want := range []string{"YOU", "VPN", "🇩🇪", "🇧🇯", "Germany Neu-Anspach", "via VPN"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() while connected missing %q in:\n%s", want, out)
		}
	}
	// The home pin stays red, the exit pin turns blue, and the startup
	// location is only a pin, never the label of the current location.
	if !strings.Contains(out, styleMarker.Render("●")) {
		t.Errorf("View() while connected missing the red home pin in:\n%s", out)
	}
	if !strings.Contains(out, styleMarkerVpn.Render("●")) {
		t.Errorf("View() while connected missing the blue exit pin in:\n%s", out)
	}
	for _, forbid := range []string{"Benin", "Abomey-Calavi"} {
		if strings.Contains(out, forbid) {
			t.Errorf("View() while connected still shows %q (startup location) in:\n%s", forbid, out)
		}
	}
}

// Without a verified exit marker the connected UI falls back to the relay's
// declared country, still labelled as a VPN exit.
func TestViewShowsRelayCountryFallbackWhileConnected(t *testing.T) {
	a := testServer("a") // JP / Japan
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
		connect: &connectState{
			server:    a,
			connected: true,
		},
	}
	out := m.View()
	for _, want := range []string{"VPN", "🇯🇵", "Japan", "via VPN"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() fallback missing %q in:\n%s", want, out)
		}
	}
}

// The globe grows with the terminal: a taller and wider terminal gets a
// bigger disc, and a narrow one still keeps the globe as long as the list
// keeps its minimum width.
func TestGlobeGrowsWithTerminal(t *testing.T) {
	big := &model{
		width:   200,
		height:  36, // rows 26 -> radius 14 (height-capped)
		servers: []vpn.Server{testServer("a")},
		results: map[string]vpn.ProbeResult{"a": {Status: vpn.ProbeWorking, LatencyMs: 42}},
	}
	gv := big.globeView()
	if gv == nil {
		t.Fatal("globeView() nil at 200x36")
	}
	if gv.width != 31 { // 2*14 + 3
		t.Errorf("globeView width = %d, want 31 at 200x36", gv.width)
	}

	narrow := &model{
		width:   70,
		height:  24,
		servers: []vpn.Server{testServer("a")},
		results: map[string]vpn.ProbeResult{"a": {Status: vpn.ProbeWorking, LatencyMs: 42}},
	}
	if gv := narrow.globeView(); gv == nil {
		t.Error("globeView() nil at 70x24, radius should still fit")
	} else if gv.width != 19 {
		t.Errorf("globeView width = %d, want 19 at 70x24", gv.width)
	}
}

// truncate must count display columns, not runes: a flag emoji is two
// columns, so cutting at a tight width must never overflow.
func TestTruncateRespectsDisplayWidth(t *testing.T) {
	jp := "\U0001F1EF\U0001F1F5" // 🇯🇵, 2 columns
	out := truncate(jp+jp+"X", 4)
	if w := lipgloss.Width(out); w > 4 {
		t.Errorf("truncate(%q, 4) = %q with %d columns, want <= 4", jp+jp+"X", out, w)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("truncate(%q, 4) should end with ellipsis, got %q", jp+jp+"X", out)
	}
	for _, s := range []string{jp, jp + jp, jp + "a"} {
		if out := truncate(s, lipgloss.Width(s)); out != s {
			t.Errorf("truncate(%q, %d) altered the string: %q", s, lipgloss.Width(s), out)
		}
	}
}

// The title bar pads its content with one space per side; it must never
// exceed the terminal width after rendering.
func TestTitleBarNeverExceedsWidth(t *testing.T) {
	m := &model{
		width:   60,
		height:  24,
		servers: []vpn.Server{testServer("a")},
		results: map[string]vpn.ProbeResult{"a": {Status: vpn.ProbeWorking, LatencyMs: 42}},
	}
	for _, w := range []int{40, 60, 80, 100, 140} {
		m.width = w
		if vw := lipgloss.Width(m.titleBar(w)); vw > w {
			t.Errorf("titleBar at width %d renders %d columns: %q", w, vw, m.titleBar(w))
		}
	}
}

// The connection log panel keeps the globe column on the right and never
// exceeds the terminal width.
func TestConnPanelKeepsGlobeColumn(t *testing.T) {
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
		geo: geoInfo{
			loaded: true,
			code:   "FR",
			name:   "France",
			city:   "Paris",
		},
		connect: &connectState{
			server:    a,
			connected: true,
			exitIP:    "185.250.249.92",
			exitCC:    "DE",
			exitGeo:   "Germany Neu-Anspach",
			lines:     []string{"[vpngate] connected via " + a.HostName, "[vpngate] exit IP: 185.250.249.92"},
		},
		connPanel: true,
	}
	out := m.View()
	for _, want := range []string{"VPN", "🇩🇪", "Germany Neu-Anspach", "exit IP: 185.250.249.92"} {
		if !strings.Contains(out, want) {
			t.Errorf("connPanel View() missing %q in:\n%s", want, out)
		}
	}
	for _, l := range strings.Split(out, "\n") {
		if vw := lipgloss.Width(l); vw > 140 {
			t.Errorf("connPanel line exceeds 140 columns: %d -> %q", vw, l)
		}
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

// buildModel returns a select-mode model with two working servers and one
// down server, sized 80x24, ready for key tests.
func buildModel(connectFn func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error) *model {
	return &model{
		servers: []vpn.Server{testServer("good"), testServer("bad")},
		results: map[string]vpn.ProbeResult{
			"good": {Status: vpn.ProbeWorking, LatencyMs: 20},
			"bad":  {Status: vpn.ProbeUnreachable},
		},
		mode:       ModeSelect,
		cursorHost: "good",
		width:      80,
		height:     24,
		round:      1,
		ctx:        context.Background(),
		connectFn:  connectFn,
	}
}

func TestEnterBlockedOnDownServer(t *testing.T) {
	started := false
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		started = true
		return nil
	})

	// Move the cursor to the down server.
	m.move(1)
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	if started {
		t.Error("connection started on a down server")
	}
	if m.blockMsg == "" {
		t.Error("expected a block message for a down server")
	}
	if m.connect != nil {
		t.Error("no connection state expected")
	}
}

func TestEnterBlockedWhileChecking(t *testing.T) {
	started := false
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		started = true
		return nil
	})
	m.results["good"] = vpn.ProbeResult{Status: vpn.ProbeChecking}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	if started {
		t.Error("connection started while the server is still being evaluated")
	}
	if m.blockMsg == "" {
		t.Error("expected a block message while checking")
	}
}

func TestEnterAllowsWorkingServer(t *testing.T) {
	startedCh := make(chan struct{}, 1)
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		startedCh <- struct{}{}
		<-ctx.Done()
		return nil
	})

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	select {
	case <-startedCh:
	case <-time.After(2 * time.Second):
		t.Error("connection not started on a working server")
	}
	if m.connect == nil || m.connect.server.HostName != "good" {
		t.Errorf("connect state missing or wrong server: %+v", m.connect)
	}
	if m.connPanel {
		t.Error("list must stay the base view after Enter")
	}
}

func TestLeftRightTogglesLogPanel(t *testing.T) {
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		<-ctx.Done()
		return nil
	})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})

	// Left opens the panel.
	m.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if !m.connPanel {
		t.Error("left arrow did not open the log panel")
	}

	// The panel view shows the log content; the list view does not.
	m.connect.lines = []string{"Initialization Sequence Completed", "tunnel verified: HTTPS 204"}
	if !strings.Contains(m.View(), "Initialization Sequence Completed") {
		t.Error("panel view does not render log lines")
	}

	// Right closes the panel back to the list.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.connPanel {
		t.Error("right arrow did not close the log panel")
	}
	if strings.Contains(m.View(), "Initialization Sequence Completed") {
		t.Error("list view must not render log lines")
	}
	if !strings.Contains(m.View(), "good") {
		t.Error("list view must remain visible after closing the panel")
	}
}

func TestConnectedRowGetsMarker(t *testing.T) {
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		<-ctx.Done()
		return nil
	})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.connect.connected = true
	m.connect.lines = []string{"Initialization Sequence Completed"}

	view := m.View()
	if !strings.Contains(view, "▶") {
		t.Error("connected row should show the ▶ marker")
	}
	if !strings.Contains(view, "connected") {
		t.Error("footer should show the connected status")
	}
}

func TestStopClearsConnectionState(t *testing.T) {
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		<-ctx.Done()
		return nil
	})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.connect.connected = true
	m.connPanel = true

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if m.connect == nil || !m.connect.canceled {
		t.Error("q should request a stop and keep state until openvpn exits")
	}

	// Once openvpn has exited, the done message clears the state and the
	// panel, returning the user to the bare list.
	nm, _ := m.Update(connMsg{done: true})
	if nm.(*model).connect != nil {
		t.Error("done message must clear the connection state")
	}
	if nm.(*model).connPanel {
		t.Error("done message must close the log panel")
	}
}

func TestPanelScrollClamps(t *testing.T) {
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		<-ctx.Done()
		return nil
	})
	m.connect = &connectState{server: m.servers[0]}
	m.connect.lines = []string{"l1", "l2", "l3"}

	m.connScroll(5)
	if m.connBottom != 2 {
		t.Errorf("scroll past top clamped to %d, want 2", m.connBottom)
	}
	m.connScroll(-10)
	if m.connBottom != 0 {
		t.Errorf("scroll past bottom clamped to %d, want 0", m.connBottom)
	}
	m.connScrollHome()
	if m.connBottom != 2 {
		t.Errorf("home should jump to the oldest line, got %d", m.connBottom)
	}
	m.connScrollEnd()
	if m.connBottom != 0 {
		t.Errorf("end should pin to the newest line, got %d", m.connBottom)
	}
}

// TestApplyConnLineTracksRealRelay verifies the streamed markers update the
// connection state: which relay really initialized the tunnel, the verified
// exit geolocation, and tunnel drops.
func TestApplyConnLineTracksRealRelay(t *testing.T) {
	m := buildModel(func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error {
		return nil
	})
	m.connect = &connectState{server: m.servers[0]}
	m.connect.connected = false

	m.applyConnLine("[vpngate] connected via bad")
	if !m.connect.connected {
		t.Error("connected marker did not flag the tunnel as up")
	}
	if m.connect.server.HostName != "bad" {
		t.Errorf("relay not tracked: connection shows %s", m.connect.server.HostName)
	}

	m.applyConnLine("[vpngate] exit: 219.100.37.63 · JP Japan")
	if m.connect.exitIP != "219.100.37.63" || m.connect.exitCC != "JP" || m.connect.exitGeo != "Japan" {
		t.Errorf("exit geo not parsed: %+v", m.connect)
	}

	m.applyConnLine("[vpngate] tunnel dropped; reconnecting…")
	if m.connect.connected {
		t.Error("tunnel still flagged up after drop marker")
	}
}

// TestParseExitLine verifies the exit marker payload split.
func TestParseExitLine(t *testing.T) {
	ip, cc, geo, ok := parseExitLine("219.100.37.63 · JP Japan")
	if !ok || ip != "219.100.37.63" || cc != "JP" || geo != "Japan" {
		t.Fatalf("bad parse: %q %q %q %v", ip, cc, geo, ok)
	}
	if _, _, _, ok := parseExitLine("garbage"); ok {
		t.Fatal("garbage accepted")
	}
}
