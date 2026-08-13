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

// truncate shortens s to at most max runes, adding an ellipsis.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}
