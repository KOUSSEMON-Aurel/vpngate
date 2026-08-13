// Package tui renders a live server browser/picker backed by a vpn.Monitor
// so servers that are verified as actually usable are always sorted first
// and re-verified in the background.
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// Mode controls whether the TUI acts as a connect picker (Enter selects and
// quits) or a read-only browser (Enter does nothing, q quits).
type Mode int

const (
	// ModeBrowse lists servers with live status, read-only.
	ModeBrowse Mode = iota
	// ModeSelect lists servers with live status; Enter picks one and quits.
	ModeSelect
)

// Options configures the TUI.
type Options struct {
	Servers     []vpn.Server
	Concurrency int
	Timeout     time.Duration
	Interval    time.Duration
	Mode        Mode
	// Watch re-verifies servers in the background. When false only a
	// single initial round runs.
	Watch bool
}

type statusMsg struct {
	round   uint64
	results map[string]vpn.ProbeResult
}

type model struct {
	servers     []vpn.Server
	monitor     *vpn.Monitor
	results     map[string]vpn.ProbeResult
	round       uint64
	cursor      int
	offset      int
	workingOnly bool
	mode        Mode
	width       int
	height      int
	selected    *vpn.Server
	quitting    bool
}

var (
	styleHeader  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	styleWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleDown    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleCursor  = lipgloss.NewStyle().Background(lipgloss.Color("238"))
)

// statusIcon maps a probe status to a single visual marker.
func statusIcon(status vpn.ProbeStatus) string {
	switch status {
	case vpn.ProbeWorking:
		return "●" // green
	case vpn.ProbeAuthFailed:
		return "◐"
	case vpn.ProbeChecking:
		return "…"
	case vpn.ProbeUnreachable:
		return "○"
	case vpn.ProbeTimeout:
		return "◌"
	default:
		return "?"
	}
}

func statusStyle(status vpn.ProbeStatus) lipgloss.Style {
	switch status {
	case vpn.ProbeWorking:
		return styleWorking
	case vpn.ProbeAuthFailed:
		return styleWarn
	case vpn.ProbeUnreachable, vpn.ProbeTimeout, vpn.ProbeError:
		return styleDown
	default:
		return styleDim
	}
}

// countryFlag converts an ISO 3166-1 alpha-2 code into its flag emoji.
func countryFlag(code string) string {
	code = strings.ToUpper(code)
	if len(code) != 2 {
		return ""
	}
	var b strings.Builder
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return ""
		}
		b.WriteRune(0x1F1E6 + (r - 'A'))
	}
	return b.String()
}

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
		// All servers are shown immediately so the list is usable before
		// the first probe round completes. Working relays sort to the top
		// as they are verified, and "w" still filters to working-only.
		workingOnly: false,
	}

	p := tea.NewProgram(m, tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		return vpn.Server{}, false, err
	}

	if m.selected == nil {
		return vpn.Server{}, false, nil
	}
	return *m.selected, true, nil
}

func (m *model) Init() tea.Cmd {
	return pollCmd(m.monitor, m.round)
}

func pollCmd(mon *vpn.Monitor, lastRound uint64) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(200 * time.Millisecond)
		return statusMsg{round: mon.Round(), results: mon.Results()}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.listLen()-1 {
				m.cursor++
			}
		case "pgup", "ctrl+b":
			m.cursor -= visibleHeight(m)
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown", "ctrl+f":
			m.cursor += visibleHeight(m)
			if m.cursor >= m.listLen() {
				m.cursor = m.listLen() - 1
			}
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = m.listLen() - 1
		case "w":
			m.workingOnly = !m.workingOnly
			m.cursor = 0
		case "r":
			m.monitor.ForceRound()
		case "enter":
			if m.mode == ModeSelect && m.listLen() > 0 {
				servers := m.displayServers()
				m.selected = &servers[m.cursor]
				return m, tea.Quit
			}
		}

	case statusMsg:
		// Apply the latest results snapshot on every poll so statuses
		// stream in live while a round is still in flight, instead of
		// waiting for the whole round to complete. The round counter only
		// advances when a full round finishes.
		if msg.round > m.round {
			m.round = msg.round
		}
		m.results = msg.results
		return m, pollCmd(m.monitor, m.round)
	}

	return m, nil
}

func visibleHeight(m *model) int {
	if m.height < 5 {
		return 10
	}
	return m.height - 4
}

// listLen is the number of rows the cursor can move over.
func (m *model) listLen() int {
	return len(m.displayServers())
}

func (m *model) workingCount() int {
	n := 0
	for _, s := range m.servers {
		if r, ok := m.results[s.HostName]; ok && r.Status == vpn.ProbeWorking {
			n++
		}
	}
	return n
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}

	servers := m.displayServers()

	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf(" %s  %-36s %-8s %-11s %6s %5s",
		"Status", "Hostname", "Country", "Latency", "Score", "")))
	b.WriteString("\n")

	for i, s := range servers {
		r := m.results[s.HostName]
		icon := statusStyle(r.Status).Render(statusIcon(r.Status))
		latency := "-"
		if r.LatencyMs > 0 {
			latency = fmt.Sprintf("%dms", r.LatencyMs)
		}

		flag := countryFlag(s.CountryShort)
		country := s.CountryLong
		if len(country) > 10 {
			country = country[:10]
		}

		status := string(r.Status)
		if r.Status == "" {
			status = "pending"
		}
		status = statusStyle(r.Status).Render(status)

		line := fmt.Sprintf(" %s  %-36s %s %-8s %-11s %6s %5d",
			icon, s.HostName, flag, country, status, latency, s.Score)

		if m.cursor == i {
			line = ">" + styleCursor.Render(line)
		} else {
			line = " " + line
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	action := "[q] quit"
	if m.mode == ModeSelect {
		action = "[enter] connect"
	}
	footer := fmt.Sprintf(" %s working · round %d · %s  [↑/↓ j/k move] [w] working-only: %v  [r] re-verify",
		styleWorking.Render(fmt.Sprint(m.workingCount())), m.round, action, m.workingOnly)
	b.WriteString(styleDim.Render(footer))
	b.WriteString("\n")

	return b.String()
}

// displayServers returns the servers for the current view: working servers
// first sorted by real latency, then the rest, optionally filtered to only
// verified-working ones.
func (m *model) displayServers() []vpn.Server {
	servers := make([]vpn.Server, len(m.servers))
	copy(servers, m.servers)

	sort.SliceStable(servers, func(i, j int) bool {
		ri, rj := m.results[servers[i].HostName], m.results[servers[j].HostName]
		ki, kj := statusRank(ri.Status), statusRank(rj.Status)
		if ki != kj {
			return ki < kj
		}
		if ki == 0 {
			return ri.LatencyMs < rj.LatencyMs
		}
		return servers[i].HostName < servers[j].HostName
	})

	if !m.workingOnly {
		return servers
	}

	out := make([]vpn.Server, 0, len(servers))
	for _, s := range servers {
		if r, ok := m.results[s.HostName]; ok && r.Status == vpn.ProbeWorking {
			out = append(out, s)
		}
	}
	return out
}

func statusRank(status vpn.ProbeStatus) int {
	switch status {
	case vpn.ProbeWorking:
		return 0
	case vpn.ProbeAuthFailed:
		return 1
	case vpn.ProbeChecking:
		return 2
	default:
		return 3
	}
}
