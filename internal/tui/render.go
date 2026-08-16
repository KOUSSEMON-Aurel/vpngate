package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davegallant/vpngate/pkg/vpn"
)

func (m *model) View() string {
	if m.quitting {
		return ""
	}

	// bubbletea renders before the first WindowSizeMsg arrives. At that
	// point our size is still 0, so falling back to a guessed width/height
	// would emit lines wider than the terminal and wrap/scroll the whole
	// screen out of alignment. Hold the frame until the real size is known.
	if m.height == 0 {
		return ""
	}

	w := m.width
	if w <= 0 {
		w = 100
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	// The connection log panel replaces the list body but keeps the title
	// bar, and is toggled with the left/right arrows. The list remains the
	// base view at all times.
	if m.connect != nil && m.connPanel {
		return m.connPanelView(w)
	}

	list := m.displayServers()
	m.ensureOffset(len(list))
	fl := m.frameLayout()

	var b strings.Builder
	b.WriteString(m.titleBar(w))
	if fl.status {
		b.WriteString(m.statusBar(w))
	}
	if fl.sep {
		b.WriteString(styleSeparator.Render(strings.Repeat("─", w)) + "\n")
	}

	mv := m.globeView()
	bodyW := w
	if mv != nil {
		bodyW = w - mv.width - 1
		if bodyW < 40 {
			bodyW = w
			mv = nil
		}
	}

	body := m.body(list, fl, bodyW)
	if mv != nil {
		body = sideBySide(body, strings.Join(mv.lines, "\n"), bodyW)
	}
	b.WriteString(body)
	if fl.footer {
		b.WriteString(m.footer(w))
	}
	if fl.geoBar {
		b.WriteString(m.geoBar(w))
	}

	// bubbletea's renderer splits View() output on "\n" and, when it is
	// taller than the terminal, drops lines from the TOP to fit. A trailing
	// newline would create an empty last split line, pushing the title bar
	// into the dropped region. Trim it so the frame is exactly h lines.
	// frameLayout() already trims chrome so the frame never exceeds h, but
	// the TrimRight keeps the final line from being an empty void.
	return strings.TrimRight(b.String(), "\n")
}

// frameLayout describes which chrome lines fit at the current terminal height
// and how many list rows remain. Terminal chrome (title, status, separator,
// column header, blank spacer, detail pane, footer, location bar) costs a
// fixed number of lines each. As the terminal shrinks, the least important
// chrome is dropped so that at least one list row always fits and the total
// frame never exceeds the height (which would make bubbletea drop the title
// from the top).
type frameLayout struct {
	rows   int
	title  bool
	status bool
	sep    bool
	header bool
	blank  bool
	detail bool
	footer bool
	geoBar bool
}

func (m *model) frameLayout() frameLayout {
	h := m.height
	if h <= 0 {
		h = 24
	}
	fl := frameLayout{
		title:  true,
		status: true,
		sep:    true,
		header: true,
		blank:  true,
		detail: h >= 16,
		footer: true,
		geoBar: true,
	}
	// On every pass, drop the least important chrome until a row fits or
	// nothing is left to drop.
	for {
		fl.rows = h - frameChrome(fl)
		if fl.rows >= 1 {
			break
		}
		switch {
		case fl.geoBar:
			fl.geoBar = false
		case fl.footer:
			fl.footer = false
		case fl.blank:
			fl.blank = false
		case fl.detail:
			fl.detail = false
		case fl.header:
			fl.header = false
		case fl.sep:
			fl.sep = false
		case fl.status:
			fl.status = false
		case fl.title:
			fl.title = false
		default:
			break
		}
	}
	if fl.rows < 1 {
		fl.rows = 1
	}
	return fl
}

// frameHeight is the total rendered lines for a layout (chrome + rows). At any
// real terminal height frameLayout() guarantees this stays <= the height so the
// title is never pushed off the top.
func frameHeight(fl frameLayout) int {
	return frameChrome(fl) + fl.rows
}

// frameChrome counts the non-list lines the frame currently needs.
func frameChrome(fl frameLayout) int {
	n := 0
	if fl.title {
		n++
	}
	if fl.status {
		n++
	}
	if fl.sep {
		n++
	}
	if fl.header {
		n++
	}
	if fl.blank {
		n++
	}
	if fl.detail {
		n += 5
	}
	if fl.footer {
		n++
	}
	if fl.geoBar {
		n++
	}
	return n
}

// body renders the column header, the visible rows and, when the layout has
// room, the detail pane for the selected server.
func (m *model) body(list []vpn.Server, fl frameLayout, w int) string {
	var b strings.Builder
	if fl.header {
		b.WriteString(m.columnHeader(w))
	}
	for i := m.offset; i < len(list) && i < m.offset+fl.rows; i++ {
		b.WriteString(m.row(list[i], i, w))
	}
	if fl.blank {
		b.WriteString("\n")
	}
	if fl.detail {
		b.WriteString(m.detailPane(w))
	}
	return b.String()
}

// sideBySide joins two multi-line blocks into a single line-per-row string,
// padding the left block to leftW so the two columns align.
func sideBySide(left, right string, leftW int) string {
	ll := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rl := strings.Split(strings.TrimRight(right, "\n"), "\n")
	pad := lipgloss.NewStyle().Width(leftW)
	n := len(ll)
	if len(rl) > n {
		n = len(rl)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		var l, r string
		if i < len(ll) {
			l = ll[i]
		}
		if i < len(rl) {
			r = rl[i]
		}
		b.WriteString(pad.Render(l))
		if i < len(rl) {
			b.WriteString(" ")
			b.WriteString(r)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// geoBar is the bottom line showing the user's resolved public location. It
// switches to the verified tunnel exit while a connection is up.
func (m *model) geoBar(w int) string {
	g := m.liveGeoInfo()
	label := g.locLabel()
	st := styleDim
	if g.loaded && g.err == nil && g.code != "" {
		st = styleGeo
	}
	return st.Render(truncate("  "+label, w)) + "\n"
}

func (m *model) titleBar(w int) string {
	mode := "browse"
	if m.mode == ModeSelect {
		mode = "connect"
	}
	watch := "watch on"
	if !m.watch {
		watch = "watch off"
	}
	left := " vpngate "
	right := fmt.Sprintf(" %s · %d relays · %s ", mode, len(m.servers), watch)
	if m.sortMode != sortModeDefault {
		right = fmt.Sprintf(" %s · %d relays · %s · sort %s ", mode, len(m.servers), watch, sortModeName(m.sortMode))
	}
	badge := m.privBadge()
	// Reserve room for the badge at the right edge so it survives
	// truncation: only the informative segment shrinks.
	full := left + right
	infoW := w - lipgloss.Width(badge)
	// Render before truncating: styleTitle pads with one space on each side,
	// and truncating first would push the line past the terminal width.
	return truncate(styleTitle.Render(full), infoW) + badge + "\n"
}

// privBadge renders the privilege state as a short badge, or "" when it
// carries no information (e.g. unsupported platform).
func (m *model) privBadge() string {
	switch m.priv {
	case privRoot:
		return stylePrivOK.Render(" sudo ")
	case privCapNetAdmin:
		return stylePrivOK.Render(" cap_net_admin ")
	case privNone:
		return stylePrivWarn.Render(" no sudo ")
	default:
		return ""
	}
}

func (m *model) statusBar(w int) string {
	done, working, checking, down := m.statusCounts()

	var b strings.Builder
	b.WriteString(" ")
	b.WriteString(styleWorking.Render(fmt.Sprintf("● %d working", working)))
	b.WriteString("  ")
	if checking > 0 {
		b.WriteString(styleChecking.Render(spinnerFrames[m.spin]))
		b.WriteString(styleChecking.Render(fmt.Sprintf(" %d verifying", checking)))
		b.WriteString("  ")
	}
	if down > 0 {
		b.WriteString(styleDown.Render(fmt.Sprintf("○ %d unreachable", down)))
		b.WriteString("  ")
	}
	b.WriteString(styleDim.Render(fmt.Sprintf("round %d", m.round)))
	b.WriteString("  ")
	b.WriteString(progressBar(done, len(m.servers)))
	if m.workingOnly {
		b.WriteString(styleChecking.Render("  working-only"))
	}
	if m.filter != "" {
		b.WriteString(styleDim.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	return truncate(b.String(), w) + "\n"
}

func (m *model) columnHeader(w int) string {
	hostW := m.hostWidth(w)
	var line string
	if w < 64 {
		line = fmt.Sprintf(" %-*s %-11s %s%-7s %-7s %-9s %-8s",
			hostW, "HOSTNAME",
			"STATUS",
			"", "COUNTRY",
			"LATENCY",
			"PROTOCOL",
			"PROVIDER",
		)
	} else {
		line = fmt.Sprintf(" %-*s %-11s %s%-9s %-8s %-7s %-9s %-8s",
			hostW, "HOSTNAME",
			"STATUS",
			"", "COUNTRY",
			"LATENCY",
			"SCORE",
			"PROTOCOL",
			"PROVIDER",
		)
	}
	return styleHeader.Render(truncate(line, w)) + "\n"
}

func (m *model) hostWidth(w int) int {
	var fixed int
	if w < 64 {
		// Compact layout: drop the score column, tighten country/latency so
		// the list still fits next to the world map on narrow terminals.
		fixed = 1 + 1 + 1 + 1 + 1 + colStatusW + 1 + 2 + 7 + 1 + 7 + 1 + 9 + 8
	} else {
		fixed = 1 + 1 + colStatusW + colCountryW + colLatencyW + colScoreW + 1 + 9 + 4 + 9
	}
	hostW := w - fixed
	if hostW < 10 {
		hostW = 10
	}
	if hostW > 40 {
		hostW = 40
	}
	return hostW
}

// sourceLabel renders a server's source for the list column, falling back to
// a dash when the source is unknown (e.g. hand-built servers in tests).
func sourceLabel(source string) string {
	if source == "" {
		return "-"
	}
	return source
}

// protocolLabel renders a server's supported VPN protocols for the list
// column, falling back to a dash when none can be determined.
func protocolLabel(s vpn.Server) string {
	if p := s.ProtocolLabel(); p != "" {
		return p
	}
	return "-"
}

// transportLabel renders a server's default OpenVPN transport for the
// detail pane, falling back to a dash when none can be determined.
func transportLabel(s vpn.Server) string {
	if t := s.TransportLabel(); t != "" {
		return t
	}
	return "-"
}

// spinnerFor returns the animated frame for a server whose state is not yet
// known, or the static status icon otherwise.
func (m *model) spinnerFor(r vpn.ProbeResult) string {
	if r.Status == vpn.ProbeChecking || r.Status == "" {
		return spinnerFrames[m.spin]
	}
	return statusIcon(r.Status)
}

func (m *model) row(s vpn.Server, i int, w int) string {
	r := m.results[s.HostName]
	info := statusInfoFor(r.Status)

	icon := info.style.Render(m.spinnerFor(r))

	latency := "-"
	if r.LatencyMs > 0 {
		latency = fmt.Sprintf("%dms", r.LatencyMs)
	}

	flag := countryFlag(s.CountryShort)
	countryMax := 9
	if w < 64 {
		countryMax = 7
	}
	country := truncate(s.CountryLong, countryMax)

	hostW := m.hostWidth(w)
	host := truncate(s.HostName, hostW)
	status := info.style.Render(fmt.Sprintf("%-11s", info.label))

	selected := m.cursorIndex() == i
	marker := " "
	// A live connection pins a marker to its server's row so the tunnel's
	// target is visible while browsing the list.
	if m.connect != nil && m.connect.server.HostName == s.HostName {
		switch status, st := m.connStatus(); status {
		case "connected":
			marker = styleWorking.Render("▶")
		case "failed":
			marker = styleDown.Render("✖")
		case "stopping…":
			marker = styleWarn.Render("◼")
		default:
			marker = st.Render(spinnerFrames[m.spin])
		}
	} else if selected {
		marker = styleHeader.Render("▸")
	}

	var line string
	if w < 64 {
		line = fmt.Sprintf(" %s %s %-*s %s %s%-7s %-7s %-9s %-8s",
			marker, icon, hostW, host,
			status,
			flag, country,
			latency,
			protocolLabel(s),
			sourceLabel(s.Source),
		)
	} else {
		line = fmt.Sprintf(" %s %s %-*s %s %s%-9s %-8s %-7d %-9s %-8s",
			marker, icon, hostW, host,
			status,
			flag, country,
			latency, s.Score,
			protocolLabel(s),
			sourceLabel(s.Source),
		)
	}

	if selected {
		line = styleSelected.Render(truncate(line, w))
	} else if i%2 == 0 {
		line = styleCursor.Render(truncate(line, w))
	} else {
		line = truncate(line, w)
	}
	return line + "\n"
}

// detailPane shows what the selected server is, what probe found and a
// plain-language verdict so the user can tell at a glance whether it works.
func (m *model) detailPane(w int) string {
	list := m.displayServers()
	if len(list) == 0 {
		return styleDim.Render(truncate("  no servers match", w)) + "\n\n\n"
	}
	idx := m.cursorIndex()
	if idx >= len(list) {
		idx = 0
	}
	s := list[idx]
	r := m.results[s.HostName]
	info := statusInfoFor(r.Status)

	icon := info.style.Render(m.spinnerFor(r))
	flag := countryFlag(s.CountryShort)
	latency := "-"
	if r.LatencyMs > 0 {
		latency = fmt.Sprintf("%dms", r.LatencyMs)
	}

	verdict := m.verdict(s, r)
	status := info.style.Render(info.label)

	var b strings.Builder
	title := fmt.Sprintf(" %s %s %s %s", icon, s.HostName, flag, truncate(s.CountryLong, 12))
	b.WriteString(styleHeader.Render(truncate(title, w)) + "\n")
	line2 := fmt.Sprintf("   ip %-15s  protocol %-9s  transport %-7s  score %-8d  latency %s  provider %-8s  %s", truncate(s.IPAddr, 15), protocolLabel(s), transportLabel(s), s.Score, latency, sourceLabel(s.Source), status)
	b.WriteString(truncate(line2, w) + "\n")
	line3 := "   verdict  " + info.style.Render(verdict)
	b.WriteString(truncate(line3, w) + "\n")
	speed := s.SpeedLabel()
	if speed == "" {
		speed = "-"
	}
	relay := fmt.Sprintf("   speed %-10s  sessions %-6d  uptime %-12s", speed, s.NumVpnSessions, s.UptimeLabel())
	b.WriteString(truncate(relay, w) + "\n")
	if s.Operator != "" {
		op := "   operator " + truncate(s.Operator, w-12)
		b.WriteString(truncate(op, w) + "\n")
	}
	return b.String()
}

func (m *model) verdict(s vpn.Server, r vpn.ProbeResult) string {
	// WARP is never actually probed; its working status means the tunnel
	// is available without any relay verification.
	if s.Source == vpn.SourceWarp && r.Status == vpn.ProbeWorking {
		return "Cloudflare WARP · no relay probe needed"
	}
	switch r.Status {
	case vpn.ProbeWorking:
		return "reachable · OpenVPN handshake OK"
	case vpn.ProbeAuthFailed:
		return "reachable · credentials refused"
	case vpn.ProbeChecking:
		return "probe in flight…"
	case vpn.ProbeUnreachable:
		if r.Detail != "" {
			return truncate(r.Detail, 50)
		}
		return "host not reachable"
	case vpn.ProbeTimeout:
		if r.Detail != "" {
			return "no handshake · " + truncate(r.Detail, 44)
		}
		return "no handshake within timeout"
	case vpn.ProbeError:
		if r.Detail != "" {
			return truncate(r.Detail, 55)
		}
		return "probe error"
	default:
		return "waiting to probe…"
	}
}

func (m *model) footer(w int) string {
	var b strings.Builder
	if m.searching {
		b.WriteString(" search: " + m.filter + "▌  [esc] clear")
	} else if m.blockMsg != "" {
		b.WriteString(styleWarn.Render(" ⚠ " + m.blockMsg))
	} else if m.connect != nil && !m.connPanel {
		status, st := m.connStatus()
		s := m.connect.server
		b.WriteString(" ")
		b.WriteString(st.Render("● " + status))
		b.WriteString(styleDim.Render(fmt.Sprintf(" %s", s.HostName)))
		if m.connect.connected {
			ip := m.connect.exitIP
			if ip == "" {
				ip = s.IPAddr
			}
			b.WriteString(styleDim.Render(fmt.Sprintf(" (%s)", ip)))
			if m.connect.exitGeo != "" {
				b.WriteString(styleDim.Render(fmt.Sprintf(" %s %s", countryFlag(m.connect.exitCC), m.connect.exitGeo)))
			}
		}
		// Probe rounds are frozen while the tunnel is up (they would run
		// through it and self-drop every relay); the tray shows that the
		// statuses on screen are the last real-path measurements.
		b.WriteString(styleDim.Render("  [←] logs"))
		b.WriteString(styleDim.Render("  [q] stop"))
		if m.healthPause != nil {
			label := "on"
			if m.healthPause.Load() {
				label = "off"
			}
			b.WriteString(styleDim.Render(fmt.Sprintf("  [p] health %s", label)))
		}
	} else {
		b.WriteString(" [↑/↓ j/k] move")
		if m.mode == ModeSelect {
			b.WriteString("  [enter] connect")
		}
		b.WriteString("  [w] working-only")
		b.WriteString(fmt.Sprintf("  [s] sort: %s", sortModeName(m.sortMode)))
		b.WriteString("  [r] re-check")
		b.WriteString("  [/] search")
		b.WriteString("  [q] quit")
	}
	return styleDim.Render(truncate(b.String(), w)) + "\n"
}
