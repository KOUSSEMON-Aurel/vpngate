package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// maxConnLines is the number of trailing openvpn output lines kept in the
// connection view.
const maxConnLines = 400

// startConnect begins an in-TUI connection for the server under the cursor
// and returns the poll cmd that feeds output lines back as messages.
// Requires a non-nil Options.ConnectFn.
func (m *model) startConnect() tea.Cmd {
	if m.connectFn == nil {
		return nil
	}
	list := m.displayServers()
	if len(list) == 0 {
		return nil
	}
	server := list[m.cursorIndex()]

	connCtx, cancel := context.WithCancel(m.ctx)
	pipe := make(chan connMsg, 256)
	m.connect = &connectState{server: server}
	m.connCancel = cancel
	m.connPipe = pipe
	m.spin = 0

	go func() {
		emit := func(line string) {
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

// stopConnect requests that the running connection be torn down. It stays
// on the connection view (marking it canceled) until openvpn has fully
// exited, then returns to the picker.
func (m *model) stopConnect() {
	if m.connCancel == nil {
		return
	}
	if m.connect != nil && !m.connect.canceled {
		m.connect.canceled = true
	}
	m.connCancel()
}

// connectView renders the live connection screen.
func (m *model) connectView(w int) string {
	cs := m.connect
	if cs == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.titleBar(w))
	b.WriteString("\n")

	s := cs.server
	flag := countryFlag(s.CountryShort)
	status, st := "connecting…", styleChecking
	switch {
	case cs.err != nil:
		status, st = "failed", styleDown
	case cs.canceled:
		status, st = "stopping…", styleWarn
	case cs.connected:
		status, st = "connected", styleWorking
	}
	head := fmt.Sprintf(" %s %s %s %s", spinnerFrames[m.spin], s.HostName, flag, st.Render(status))
	b.WriteString(truncate(head, w) + "\n")

	if cs.err != nil && !cs.canceled {
		b.WriteString(styleError.Render(truncate(" ✖ "+cs.err.Error(), w)) + "\n")
	}

	b.WriteString(styleSeparator.Render(strings.Repeat("─", w)) + "\n")

	chrome := 3 // title, head, separator
	if cs.err != nil && !cs.canceled {
		chrome++
	}
	avail := m.height - chrome
	lines := cs.lines
	if avail < 0 {
		avail = 0
	}
	if len(lines) > avail {
		lines = lines[len(lines)-avail:]
	}
	if len(lines) == 0 {
		b.WriteString(styleDim.Render(truncate(" waiting for openvpn output…", w)) + "\n")
	} else {
		for _, l := range lines {
			b.WriteString(styleDim.Render(truncate(l, w)) + "\n")
		}
	}

	hint := "[q] stop and return to picker   [ctrl+c] quit"
	if cs.canceled {
		hint = "stopping openvpn…   [ctrl+c] quit"
	}
	if cs.err != nil && !cs.canceled {
		hint = "connection failed   [q] back to picker   [ctrl+c] quit"
	}
	b.WriteString(styleDim.Render(truncate(" "+hint, w)) + "\n")

	return strings.TrimRight(b.String(), "\n")
}
