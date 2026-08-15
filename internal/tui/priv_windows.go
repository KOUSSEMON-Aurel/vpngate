//go:build windows

package tui

// detectPrivilege is a stub on Windows: openvpn runs as a service/user
// there and the privilege model differs, so the TUI shows no badge.
func detectPrivilege() privState {
	return privUnknown
}
