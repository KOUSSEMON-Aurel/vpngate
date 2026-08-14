package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/davegallant/vpngate/pkg/vpn"
)

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While a connection is live, navigation input is suspended and only
	// stop/quit keys apply. openvpn receives stdin/logs elsewhere, so no
	// other key is forwarded. After a failed connection the error screen
	// is dismissed by any of q/esc/enter.
	if m.connect != nil {
		switch msg.String() {
		case "q", "esc", "enter":
			if m.connect.err != nil && !m.connect.canceled {
				m.connect = nil
			} else {
				m.stopConnect()
			}
		case "ctrl+c":
			if m.connect.err != nil && !m.connect.canceled {
				m.quitting = true
				return m, tea.Quit
			}
			m.stopConnect()
		}
		return m, m.tickIfNeeded()
	}

	switch msg.String() {
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
			if m.connectFn != nil {
				return m, m.startConnect()
			}
			list := m.displayServers()
			m.selected = &list[m.cursorIndex()]
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
	case "r":
		m.monitor.ForceRound()

	default:
		if m.searching && len(msg.String()) == 1 {
			m.filter += msg.String()
		}
	}

	return m, m.tickIfNeeded()
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
