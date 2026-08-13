package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// statusInfo describes how a probe status is rendered.
type statusInfo struct {
	label string
	style lipgloss.Style
	rank  int
}

func statusInfoFor(st vpn.ProbeStatus) statusInfo {
	switch st {
	case vpn.ProbeWorking:
		return statusInfo{"working", styleWorking, 0}
	case vpn.ProbeAuthFailed:
		return statusInfo{"auth failed", styleWarn, 1}
	case vpn.ProbeChecking:
		return statusInfo{"checking", styleChecking, 2}
	case vpn.ProbeUnreachable:
		return statusInfo{"unreachable", styleDown, 3}
	case vpn.ProbeTimeout:
		return statusInfo{"timeout", styleDown, 3}
	case vpn.ProbeError:
		return statusInfo{"error", styleError, 3}
	default:
		return statusInfo{"pending", styleDim, 4}
	}
}

// statusIcon maps a probe status to a single visual marker.
func statusIcon(status vpn.ProbeStatus) string {
	switch status {
	case vpn.ProbeWorking:
		return "●"
	case vpn.ProbeAuthFailed:
		return "◐"
	case vpn.ProbeChecking:
		return "…"
	case vpn.ProbeUnreachable:
		return "○"
	case vpn.ProbeTimeout:
		return "◌"
	case vpn.ProbeError:
		return "✕"
	default:
		return "?"
	}
}

// statusStyle returns the color used for a status label.
func statusStyle(status vpn.ProbeStatus) lipgloss.Style {
	return statusInfoFor(status).style
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

// ansiReset clears terminal styling; truncate appends it when a cut lands
// inside a styled segment so the ellipsis never leaks the truncated colour.
const ansiReset = "\x1b[0m"

// truncate shortens s to at most max visible columns, appending an ellipsis
// when it cuts. ANSI escape sequences count as zero width and are copied
// verbatim, so styled strings can be truncated without splitting an escape
// sequence (which would render as literal garbage) or corrupting colours.
func truncate(s string, max int) string {
	rs := []rune(s)
	if max <= 0 {
		return ""
	}
	if max == 1 {
		if len(rs) <= 1 {
			return s
		}
		return string(rs[:1])
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(rs); {
		r := rs[i]
		if r == '\x1b' {
			start := i
			i++
			if i < len(rs) && rs[i] == '[' {
				// CSI: consume parameters up to the terminating byte
				// (0x40-0x7E) so the sequence is never split.
				i++
				for i < len(rs) && !(rs[i] >= 0x40 && rs[i] <= 0x7e) {
					i++
				}
				if i < len(rs) {
					i++
				}
			} else {
				// OSC or lone ESC: consume up to BEL (or the end).
				for i < len(rs) && rs[i] != '\a' {
					i++
				}
				if i < len(rs) {
					i++
				}
			}
			b.WriteString(string(rs[start:i]))
			continue
		}
		if n >= max-1 {
			b.WriteString(ansiReset + "…")
			return b.String()
		}
		b.WriteRune(r)
		n++
		i++
	}
	return b.String()
}
