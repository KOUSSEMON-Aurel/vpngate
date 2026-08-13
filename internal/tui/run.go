package tui

import (
	"context"
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
		servers: opts.Servers,
		monitor: mon,
		results: make(map[string]vpn.ProbeResult),
		mode:    opts.Mode,
		watch:   opts.Watch,
	}

	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return vpn.Server{}, false, err
	}

	if m.selected == nil {
		return vpn.Server{}, false, nil
	}
	return *m.selected, true, nil
}

func (m *model) Init() tea.Cmd {
	return m.nextCmds()
}

// pollCmd samples the monitor's latest results on a fixed interval.
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

// nextCmds keeps the poll loop running and, while anything needs animating,
// the spinner tick as well.
func (m *model) nextCmds() tea.Cmd {
	cmds := []tea.Cmd{pollCmd(m.monitor, m.round)}
	if m.animated() {
		cmds = append(cmds, tickCmd())
	}
	return tea.Batch(cmds...)
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

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, m.nextCmds()

	case tickMsg:
		m.spin = (m.spin + 1) % len(spinnerFrames)
		return m, m.nextCmds()

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
		return m, m.nextCmds()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, m.nextCmds()
}
