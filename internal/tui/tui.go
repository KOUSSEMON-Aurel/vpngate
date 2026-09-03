// Package tui renders a live server browser/picker backed by a vpn.Monitor
// so servers that are verified as actually usable are always sorted first
// and re-verified in the background.
package tui

import (
	"context"
	"sync/atomic"
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
	// ConnectFn, when set, runs an in-TUI connection: Enter in select mode
	// keeps the TUI alive, streams ConnectFn's output until it returns, and
	// then returns to the picker instead of quitting. results carries the
	// latest probe verdicts so the connection can prefer relays that are
	// actually reachable.
	ConnectFn func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error
	// HealthPause, when non-nil, is the pause switch of the live-tunnel
	// watchdog. While a connection is up the p key toggles it: paused, the
	// watchdog stops probing and can never drop the tunnel. Nil hides the
	// toggle.
	HealthPause *atomic.Bool
}

// connMsg streams one output line from an in-TUI connection, or marks its
// completion. A no-field connMsg{} also serves as a poll heartbeat for the
// connection view.
type connMsg struct {
	line string
	done bool
	err  error
}

// connectState is a live in-TUI VPN connection.
type connectState struct {
	server    vpn.Server
	lines     []string
	err       error
	connected bool
	canceled  bool
	// exitIP/exitCC/exitGeo describe the real tunnel egress as reported
	// by the connection's own verification, not the server that was
	// selected in the picker (a retry may land on another relay).
	exitIP  string
	exitCC  string
	exitGeo string
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
	order        []string
	orderRound   uint64
	orderWorking int
	userMoved    bool
	lastResize   time.Time
	pendingW    int
	pendingH    int
	ctx         context.Context
	connectFn   func(ctx context.Context, server vpn.Server, results map[string]vpn.ProbeResult, emit func(string)) error
	// healthPause is the shared pause switch of the live-tunnel watchdog,
	// toggled with the p key while a connection is up. Nil hides the toggle.
	healthPause *atomic.Bool
	connect     *connectState
	connCancel  context.CancelFunc
	connPipe    chan connMsg
	// priv records how the process can create a tun interface (sudo,
	// CAP_NET_ADMIN on a binary, or not at all). Detected once at startup
	// and shown as a badge in the title bar.
	priv privState
	// connPanel shows the live connection log panel instead of the list.
	// It is toggled with the left/right arrows; the list stays the base
	// view at all times.
	connPanel bool
	// connBottom is the scroll offset of the log panel from the newest
	// line (0 = pinned to the latest output).
	connBottom int
	// blockMsg is a transient hint shown in the footer when Enter was
	// pressed on a server that cannot be connected to (down or still
	// being evaluated). Cleared on the next key.
	blockMsg string
	// sortMode cycles the list ordering with the s key: default (working
	// relays first by real latency), then the latency/score match in
	// ascending and descending order.
	sortMode int
}

// List ordering modes, cycled with the s key.
const (
	sortModeDefault = iota
	sortModeBest    // latency/score ascending: best relays first
	sortModeWorst   // latency/score descending: worst relays first
)

func sortModeName(mode int) string {
	switch mode {
	case sortModeBest:
		return "best"
	case sortModeWorst:
		return "worst"
	}
	return "default"
}

// spinnerFrames is a compact braille loader used for servers whose state is
// not known yet.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	styleTitle        = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	styleHeader       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("110"))
	styleWorking      = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleChecking     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleWarn         = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleDown         = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleError        = lipgloss.NewStyle().Foreground(lipgloss.Color("201"))
	styleDim          = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleCursor       = lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(lipgloss.Color("255"))
	styleSelected     = lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255"))
	styleSeparator    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleGeo          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45"))
	styleMarker       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")) // home pin: red
	styleMarkerVpn    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))  // exit pin: blue
	styleMarkerRim    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("88"))  // home pin behind the globe: dim red
	styleMarkerVpnRim = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("24"))  // exit pin behind the globe: dim blue
	stylePrivOK       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))   // sudo / CAP_NET_ADMIN badge: green
	stylePrivWarn     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))  // no-sudo badge: red
)

// Column widths for the fixed-width fields; the hostname takes the rest.
const (
	colStatusW   = 12 // "unreachable" + space
	colLatencyW  = 9  // "2414ms" + space
	colScoreW    = 8
	colCountryW  = 13 // flag (2) + space + 9 runes + space
	minListW     = 44 // minimum columns the server list keeps beside the globe
	listNaturalW = 90 // max columns the list body actually uses; beyond it the
	//                      leftover width belongs to the globe column
)
