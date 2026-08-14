package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// Run starts the TUI and blocks until the user quits. In select mode the
// chosen server is returned (ok == true).
func Run(ctx context.Context, opts Options) (vpn.Server, bool, error) {
	mon := vpn.NewMonitor(opts.Servers, vpn.MonitorOptions{
		Concurrency: opts.Concurrency,
		Timeout:     opts.Timeout,
		Interval:    opts.Interval,
		Continuous:  opts.Watch,
	})
	mon.Start()
	defer mon.Stop()

	m := &model{
		servers:   opts.Servers,
		monitor:   mon,
		results:   make(map[string]vpn.ProbeResult),
		mode:      opts.Mode,
		watch:     opts.Watch,
		ctx:       ctx,
		connectFn: opts.ConnectFn,
	}

	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return vpn.Server{}, false, err
	}

	// Ensure any connection started inside this run is torn down even if the
	// program exits without a clean key sequence (e.g. parent context cancel).
	if m.connCancel != nil {
		m.connCancel()
	}

	if m.selected == nil {
		return vpn.Server{}, false, nil
	}
	return *m.selected, true, nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(pollCmd(m.monitor, m.round), m.tickIfNeeded(), geoCmd())
}

// pollCmd samples the monitor's latest results on a fixed interval. The poll
// chain is self-sustaining: statusMsg always re-arms exactly one pollCmd, so
// it never multiplies.
func pollCmd(mon *vpn.Monitor, lastRound uint64) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(200 * time.Millisecond)
		return statusMsg{round: mon.Round(), results: mon.Results()}
	}
}

// tickCmd advances the spinner animation frame.
func tickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(90 * time.Millisecond)
		return tickMsg{}
	}
}

// resizeCoalesce throttles terminal resize events. While the user drags the
// window border the terminal floods the program with WindowSizeMsg; applying
// each one would rebuild the frame dozens of times per second. Events within
// the window are coalesced into a single delayed resizeMsg instead.
const resizeCoalesce = 40 * time.Millisecond

// resizeMsg applies a terminal size after the resize burst settles.
type resizeMsg struct {
	width  int
	height int
}

// applySize stores the new terminal size and marks the globe for a rebuild.
func (m *model) applySize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width, m.height = w, h
	m.globeDirty = true
}

// tickIfNeeded starts the tick chain, but only when something needs it and
// no chain is already live. Without the guard a new chain would be armed on
// every statusMsg, and since each tick re-arms itself, chains would multiply
// until the renderer saturates and key events queue up indefinitely.
func (m *model) tickIfNeeded() tea.Cmd {
	if (m.animated() || m.globeVisible()) && !m.tickAlive {
		m.tickAlive = true
		return tickCmd()
	}
	return nil
}

// animated reports whether something on screen needs the spinner to advance:
// a server still pending/checking, an active search, or results not filled
// yet.
func (m *model) animated() bool {
	if m.searching {
		return true
	}
	if len(m.results) < len(m.servers) {
		return true
	}
	for _, r := range m.results {
		if r.Status == vpn.ProbeChecking || r.Status == "" {
			return true
		}
	}
	return false
}

// globeVisible reports whether the globe column is wide/tall enough to be
// shown; while visible the tick chain keeps running so it keeps rotating.
func (m *model) globeVisible() bool {
	return m.width >= 80 && m.visibleRows() >= 7
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Coalesce resize bursts: apply immediately only when the last
		// resize was long enough ago, otherwise defer to a single
		// delayed resizeMsg carrying the newest pending size.
		m.pendingW, m.pendingH = msg.Width, msg.Height
		if time.Since(m.lastResize) < resizeCoalesce {
			return m, tea.Tick(resizeCoalesce, func(time.Time) tea.Msg {
				return resizeMsg{width: m.pendingW, height: m.pendingH}
			})
		}
		m.lastResize = time.Now()
		m.applySize(msg.Width, msg.Height)
		// The renderer diffs against its pre-resize buffer, so shrunk
		// chrome would linger on screen. Clear forces a full repaint of
		// the new layout.
		return m, tea.Batch(tea.ClearScreen, m.tickIfNeeded())

	case resizeMsg:
		m.lastResize = time.Now()
		m.applySize(msg.width, msg.height)
		return m, tea.Batch(tea.ClearScreen, m.tickIfNeeded())

	case tickMsg:
		// The chain dies on entry; if anything still needs animating it
		// is re-armed right here, exactly once.
		m.tickAlive = false
		m.spin = (m.spin + 1) % len(spinnerFrames)
		m.globeRot++
		m.globeDirty = true
		return m, m.tickIfNeeded()

	case statusMsg:
		// Apply the latest results snapshot on every poll so statuses
		// stream in live while a round is still in flight. The round
		// counter only advances when a full round finishes.
		if msg.round > m.round {
			m.round = msg.round
		}
		m.results = msg.results
		if m.cursorHost == "" && len(m.servers) > 0 {
			// Auto-follow the best server until the user moves.
			list := m.displayServers()
			if len(list) > 0 {
				m.cursorHost = list[0].HostName
			}
		}
		return m, tea.Batch(pollCmd(m.monitor, m.round), m.tickIfNeeded())

	case geoMsg:
		m.geo = geoInfo(msg)
		m.globeDirty = true
		return m, m.tickIfNeeded()

	case connMsg:
		if m.connect == nil {
			return m, m.tickIfNeeded()
		}
		if msg.done {
			// A user-cancel returns straight to the picker. A failure
			// keeps the connection screen with the error visible until
			// dismissed (q/esc/enter).
			if msg.err != nil && !m.connect.canceled {
				m.connect.err = msg.err
				m.connPipe = nil
				m.connCancel = nil
				return m, m.tickIfNeeded()
			}
			m.connPipe = nil
			m.connCancel = nil
			m.connect = nil
			return m, m.tickIfNeeded()
		}
		if msg.line != "" {
			lines := append(m.connect.lines, msg.line)
			if len(lines) > maxConnLines {
				lines = lines[len(lines)-maxConnLines:]
			}
			m.connect.lines = lines
			if strings.Contains(msg.line, "Initialization Sequence Completed") {
				m.connect.connected = true
			}
		}
		// Advance the connection spinner on each ~150ms heartbeat.
		m.spin = (m.spin + 1) % len(spinnerFrames)
		return m, m.connPollCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, m.tickIfNeeded()
}
