package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// maxConnLines is the number of trailing openvpn output lines kept for the
// connection panel.
const maxConnLines = 400

// startConnect begins an in-TUI connection for the server under the cursor
// and returns the poll cmd that feeds output lines back as messages.
// Requires a non-nil Options.ConnectFn. Any live connection is stopped
// first; the list stays on screen and the log panel can be toggled with
// the left/right arrows.
func (m *model) startConnect() tea.Cmd {
	if m.connectFn == nil {
		return nil
	}
	list := m.displayServers()
	if len(list) == 0 {
		return nil
	}
	server := list[m.cursorIndex()]

	m.stopConnect()

	connCtx, cancel := context.WithCancel(m.ctx)
	pipe := make(chan connMsg, 256)
	m.connect = &connectState{server: server}
	m.connCancel = cancel
	m.connPipe = pipe
	m.connPanel = false
	m.connBottom = 0
	m.spin = 0

	go func() {
		emit := func(line string) {
			// Once canceled, never send: a send racing with the pipe
			// being closed would panic. The done message below is the
			// only signal after teardown.
			select {
			case <-connCtx.Done():
				return
			default:
			}
			select {
			case pipe <- connMsg{line: line}:
			case <-connCtx.Done():
			}
		}
		err := m.connectFn(connCtx, server, emit)
		select {
		case pipe <- connMsg{done: true, err: err}:
		default:
		}
		close(pipe)
	}()

	return m.connPollCmd()
}

// connPollCmd re-arms itself until the connection pipe is closed.
func (m *model) connPollCmd() tea.Cmd {
	return func() tea.Msg {
		if m.connPipe == nil {
			return connMsg{done: true}
		}
		select {
		case msg, ok := <-m.connPipe:
			if !ok {
				return connMsg{done: true}
			}
			return msg
		case <-time.After(150 * time.Millisecond):
			return connMsg{}
		}
	}
}

// stopConnect requests that the running connection be torn down. The list
// stays on screen; the log panel keeps showing the shutdown output until
// openvpn has fully exited.
func (m *model) stopConnect() {
	if m.connCancel == nil {
		return
	}
	if m.connect != nil && !m.connect.canceled {
		m.connect.canceled = true
	}
	m.connCancel()
}

// connScroll shifts the log panel view; negative values scroll up (older
// lines), positive down (toward the newest).
func (m *model) connScroll(delta int) {
	m.connBottom += delta
	m.connClampScroll()
}

// connClampScroll keeps the scroll offset inside the log bounds.
func (m *model) connClampScroll() {
	if m.connect == nil {
		m.connBottom = 0
		return
	}
	if m.connBottom < 0 {
		m.connBottom = 0
	}
	max := len(m.connect.lines) - 1
	if max < 0 {
		max = 0
	}
	if m.connBottom > max {
		m.connBottom = max
	}
}

// connScrollHome jumps to the oldest buffered line.
func (m *model) connScrollHome() {
	m.connBottom = len(m.connect.lines) - 1
	m.connClampScroll()
}

// connScrollEnd pins the panel to the newest line.
func (m *model) connScrollEnd() {
	m.connBottom = 0
}

// connStatus returns the current connection status label and style.
func (m *model) connStatus() (string, lipgloss.Style) {
	cs := m.connect
	if cs == nil {
		return "", styleDim
	}
	switch {
	case cs.err != nil && !cs.canceled:
		return "failed", styleDown
	case cs.canceled:
		return "stopping…", styleWarn
	case cs.connected:
		return "connected", styleWorking
	default:
		return "connecting…", styleChecking
	}
}

// connPanelView renders the connection log panel, which replaces the list
// body while m.connPanel is set. The title bar stays on top and the panel
// keeps the same chrome budget so it never exceeds the terminal height.
func (m *model) connPanelView(w int) string {
	cs := m.connect
	if cs == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.titleBar(w))
	b.WriteString("\n")

	s := cs.server
	flag := countryFlag(s.CountryShort)
	status, st := m.connStatus()
	head := fmt.Sprintf(" %s %s %s %s", spinnerFrames[m.spin], s.HostName, flag, st.Render(status))
	b.WriteString(truncate(head, w) + "\n")

	if cs.err != nil && !cs.canceled {
		b.WriteString(styleError.Render(truncate(" ✖ "+cs.err.Error(), w)) + "\n")
	}

	b.WriteString(styleSeparator.Render(strings.Repeat("─", w)) + "\n")

	chrome := 4 // title, head, separator, hint
	if cs.err != nil && !cs.canceled {
		chrome++
	}
	avail := m.height - chrome
	if avail < 0 {
		avail = 0
	}

	lines := cs.lines
	if len(lines) == 0 {
		b.WriteString(styleDim.Render(truncate(" waiting for openvpn output…", w)) + "\n")
		avail--
	} else {
		m.connClampScroll()
		start := len(lines) - m.connBottom - avail
		if start < 0 {
			start = 0
		}
		for _, l := range lines[start:] {
			b.WriteString(styleDim.Render(truncate(l, w)) + "\n")
		}
	}

	if m.connBottom > 0 {
		b.WriteString(styleChecking.Render(truncate(fmt.Sprintf(" ↑ %d older lines hidden", m.connBottom), w)) + "\n")
	} else {
		b.WriteString(styleDim.Render(truncate(" "+m.connHint(), w)) + "\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// connHint is the key hint line for the log panel.
func (m *model) connHint() string {
	if m.connect == nil {
		return ""
	}
	if m.connect.err != nil && !m.connect.canceled {
		return "connection failed   [right] back to list   [q] clear   [ctrl+c] quit"
	}
	if m.connect.canceled {
		return "stopping openvpn…   [ctrl+c] quit"
	}
	return "[↑/↓] scroll   [right] back to list   [q] stop   [ctrl+c] quit"
}
