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

	w := m.width
	if w <= 0 {
		w = 100
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	list := m.displayServers()
	m.ensureOffset(len(list))

	var b strings.Builder
	b.WriteString(m.titleBar(w))
	b.WriteString(m.statusBar(w))
	b.WriteString(styleSeparator.Render(strings.Repeat("─", w)) + "\n")

	mv := m.globeView()
	bodyW := w
	if mv != nil {
		bodyW = w - mv.width - 1
		if bodyW < 40 {
			bodyW = w
			mv = nil
		}
	}

	body := m.body(list, h, bodyW)
	if mv != nil {
		body = sideBySide(body, strings.Join(mv.lines, "\n"), bodyW)
	}
	b.WriteString(body)
	b.WriteString(m.footer(w))
	b.WriteString(m.geoBar(w))
	return b.String()
}

// body renders the column header, the visible rows and, when the terminal is
// tall enough, the detail pane for the selected server.
func (m *model) body(list []vpn.Server, h, w int) string {
	var b strings.Builder
	b.WriteString(m.columnHeader(w))
	rows := m.visibleRows()
	for i := m.offset; i < len(list) && i < m.offset+rows; i++ {
		b.WriteString(m.row(list[i], i, w))
	}
	b.WriteString("\n")
	if h >= 16 {
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

// geoBar is the bottom line showing the user's resolved public location.
func (m *model) geoBar(w int) string {
	label := m.geo.locLabel()
	st := styleDim
	if m.geo.loaded && m.geo.err == nil && m.geo.code != "" {
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
	full := truncate(left+right, w)
	return styleTitle.Render(truncate(full, w)) + "\n"
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
		line = fmt.Sprintf(" %-*s %-11s %s%-7s %-7s",
			hostW, "HOSTNAME",
			"STATUS",
			"", "COUNTRY",
			"LATENCY",
		)
	} else {
		line = fmt.Sprintf(" %-*s %-11s %s%-9s %-8s %-7s",
			hostW, "HOSTNAME",
			"STATUS",
			"", "COUNTRY",
			"LATENCY",
			"SCORE",
		)
	}
	return styleHeader.Render(truncate(line, w)) + "\n"
}

func (m *model) hostWidth(w int) int {
	var fixed int
	if w < 64 {
		// Compact layout: drop the score column, tighten country/latency so
		// the list still fits next to the world map on narrow terminals.
		fixed = 1 + 1 + 1 + 1 + 1 + colStatusW + 1 + 2 + 7 + 1 + 7
	} else {
		fixed = 1 + 1 + colStatusW + colCountryW + colLatencyW + colScoreW + 4
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
	if selected {
		marker = styleHeader.Render("▸")
	}

	var line string
	if w < 64 {
		line = fmt.Sprintf(" %s %s %-*s %s %s%-7s %-7s",
			marker, icon, hostW, host,
			status,
			flag, country,
			latency,
		)
	} else {
		line = fmt.Sprintf(" %s %s %-*s %s %s%-9s %-8s %-7d",
			marker, icon, hostW, host,
			status,
			flag, country,
			latency, s.Score,
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
		return styleDim.Render(truncate("  no servers match", w)) + "\n\n"
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

	verdict := m.verdict(r)
	status := info.style.Render(info.label)

	var b strings.Builder
	title := fmt.Sprintf(" %s %s %s %s", icon, s.HostName, flag, truncate(s.CountryLong, 12))
	b.WriteString(styleHeader.Render(truncate(title, w)) + "\n")
	line2 := fmt.Sprintf("   ip %-15s  score %-8d  latency %s  %s", truncate(s.IPAddr, 15), s.Score, latency, status)
	b.WriteString(truncate(line2, w) + "\n")
	line3 := "   verdict  " + info.style.Render(verdict)
	b.WriteString(truncate(line3, w) + "\n")
	return b.String()
}

func (m *model) verdict(r vpn.ProbeResult) string {
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
	} else {
		b.WriteString(" [↑/↓ j/k] move")
		if m.mode == ModeSelect {
			b.WriteString("  [enter] connect")
		}
		b.WriteString("  [w] working-only")
		b.WriteString("  [r] re-check")
		b.WriteString("  [/] search")
		b.WriteString("  [q] quit")
	}
	return styleDim.Render(truncate(b.String(), w)) + "\n"
}
