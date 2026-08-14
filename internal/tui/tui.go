// Package tui renders a live server browser/picker backed by a vpn.Monitor
// so servers that are verified as actually usable are always sorted first
// and re-verified in the background.
package tui

import (
	"time"

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

type tickMsg struct{}

// model is the bubbletea state machine backing the TUI.
type model struct {
	servers     []vpn.Server
	monitor     *vpn.Monitor
	results     map[string]vpn.ProbeResult
	round       uint64
	cursorHost  string
	offset      int
	workingOnly bool
	filter      string
	searching   bool
	mode        Mode
	watch       bool
	width       int
	height      int
	selected    *vpn.Server
	quitting    bool
	spin        int
	globeRot    int
	globeCache  *globeView
	globeDirty  bool
	tickAlive   bool
	geo         geoInfo
	order       []string
	orderRound  uint64
	lastResize  time.Time
	pendingW    int
	pendingH    int
}

// spinnerFrames is a compact braille loader used for servers whose state is
// not known yet.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	styleTitle     = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	styleHeader    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	styleWorking   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleChecking  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleWarn      = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleDown      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleCursor    = lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(lipgloss.Color("255"))
	styleSelected  = lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255"))
	styleSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleGeo       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45"))
	styleMarker    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	styleMarkerVpn = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226"))
)

// Column widths for the fixed-width fields; the hostname takes the rest.
const (
	colStatusW  = 12 // "unreachable" + space
	colLatencyW = 9  // "2414ms" + space
	colScoreW   = 8
	colCountryW = 13 // flag (2) + space + 9 runes + space
)
