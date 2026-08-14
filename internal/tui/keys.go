package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/davegallant/vpngate/pkg/vpn"
)

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	m.blockMsg = ""

	// While a connection is live the list stays the base view: arrows and
	// navigation keep working, the left/right arrows toggle the log panel,
	// q stops the connection and ctrl+c quits.
	if m.connect != nil {
		switch key {
		case "ctrl+c":
			if m.connect.err != nil && !m.connect.canceled {
				m.quitting = true
				return m, tea.Quit
			}
			m.stopConnect()
			return m, m.tickIfNeeded()
		case "q":
			if m.connect.err != nil && !m.connect.canceled {
				m.connect = nil
				m.connPanel = false
				m.connBottom = 0
			} else {
				m.stopConnect()
			}
			return m, m.tickIfNeeded()
		case "left":
			m.connPanel = true
			return m, m.tickIfNeeded()
		case "right":
			m.connPanel = false
			return m, m.tickIfNeeded()
		}

		// Inside the log panel only panel keys apply: arrows scroll, esc or
		// right returns to the list.
		if m.connPanel {
			switch key {
			case "esc":
				m.connPanel = false
			case "up", "k":
				m.connScroll(1)
			case "down", "j":
				m.connScroll(-1)
			case "pgup", "ctrl+b":
				m.connScroll(m.visibleRows())
			case "pgdown", "ctrl+f":
				m.connScroll(-m.visibleRows())
			case "home":
				m.connScrollHome()
			case "end":
				m.connScrollEnd()
			}
			return m, m.tickIfNeeded()
		}

		// Outside the panel the list stays fully navigable; Enter on a
		// different server switches the tunnel target.
	}

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "q":
		if m.searching {
			m.searching = false
			m.filter = ""
			return m, m.tickIfNeeded()
		}
		m.quitting = true
		return m, tea.Quit

	case "esc":
		if m.searching {
			m.searching = false
			m.filter = ""
		}
		return m, m.tickIfNeeded()

	case "enter":
		if m.searching {
			m.searching = false
			return m, m.tickIfNeeded()
		}
		if m.mode == ModeSelect && m.listLen() > 0 {
			list := m.displayServers()
			idx := m.cursorIndex()
			if reason := m.enterBlocked(list[idx]); reason != "" {
				m.blockMsg = reason
				return m, m.tickIfNeeded()
			}
			if m.connectFn != nil {
				return m, m.startConnect()
			}
			m.selected = &list[idx]
			return m, tea.Quit
		}
		return m, m.tickIfNeeded()

	case "/":
		m.searching = true
		return m, m.tickIfNeeded()

	case "backspace":
		if m.searching {
			if r := []rune(m.filter); len(r) > 1 {
				m.filter = string(r[:len(r)-1])
			} else {
				m.filter = ""
				m.searching = false
			}
		}
		return m, m.tickIfNeeded()

	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup", "ctrl+b":
		m.move(-m.visibleRows())
	case "pgdown", "ctrl+f":
		m.move(m.visibleRows())
	case "home":
		m.cursorHost = firstHost(m.displayServers())
	case "end":
		m.cursorHost = lastHost(m.displayServers())
	case "w":
		m.workingOnly = !m.workingOnly
	case "s":
		m.sortMode = (m.sortMode + 1) % 3
		// Re-derive the order immediately instead of waiting for the next
		// probe round; an explicit user sort wins over the freeze.
		m.orderRound = ^uint64(0)
	case "r":
		m.monitor.ForceRound()

	default:
		if m.searching && len(msg.String()) == 1 {
			m.filter += msg.String()
		}
	}

	return m, m.tickIfNeeded()
}

// enterBlocked returns a non-empty reason when Enter must not connect to
// the given server: servers judged down (unreachable, timeout, auth
// failure, probe error) are red and cannot be connected to, and servers
// still being evaluated (checking, or not probed yet) are off-limits until
// their verdict lands. Working servers are always allowed.
func (m *model) enterBlocked(s vpn.Server) string {
	r := m.results[s.HostName]
	switch r.Status {
	case vpn.ProbeWorking:
		return ""
	case vpn.ProbeChecking, "":
		return s.HostName + " is still being evaluated — wait for the verdict"
	case vpn.ProbeAuthFailed:
		return s.HostName + " refused the connection (relay likely full) — pick a working server"
	case vpn.ProbeUnreachable, vpn.ProbeTimeout, vpn.ProbeError:
		return s.HostName + " is down — pick a working server"
	}
	return ""
}

func firstHost(list []vpn.Server) string {
	if len(list) == 0 {
		return ""
	}
	return list[0].HostName
}

func lastHost(list []vpn.Server) string {
	if len(list) == 0 {
		return ""
	}
	return list[len(list)-1].HostName
}

// move shifts the selection by delta rows, keeping it pinned to a specific
// host so re-sorting during live probing cannot hijack the cursor.
func (m *model) move(delta int) {
	list := m.displayServers()
	if len(list) == 0 {
		m.cursorHost = ""
		return
	}
	idx := m.cursorIndex() + delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(list) {
		idx = len(list) - 1
	}
	m.cursorHost = list[idx].HostName
}

func (m *model) listLen() int {
	return len(m.displayServers())
}

func (m *model) cursorIndex() int {
	list := m.displayServers()
	if m.cursorHost == "" || len(list) == 0 {
		return 0
	}
	for i, s := range list {
		if s.HostName == m.cursorHost {
			return i
		}
	}
	return 0
}

func (m *model) visibleRows() int {
	return m.frameLayout().rows
}

// ensureOffset keeps the cursor row visible after the list re-sorts or the
// selection moves.
func (m *model) ensureOffset(n int) {
	if n == 0 {
		m.offset = 0
		return
	}
	rows := m.visibleRows()
	if rows > n {
		rows = n
	}
	idx := m.cursorIndex()
	if idx < m.offset {
		m.offset = idx
	}
	if idx >= m.offset+rows {
		m.offset = idx - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > n-rows {
		m.offset = n - rows
	}
}
