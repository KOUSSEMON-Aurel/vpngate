//go:build !windows

package daemon

import (
	"errors"
	"os"
	"syscall"
)

// IsAlive reports whether pid identifies a running process.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// DetachAttr returns the SysProcAttr that starts a child process in its
// own session, detached from the parent's controlling terminal and
// process group, so it survives the parent exiting.
func DetachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// defaultBaseDir is the fixed, machine-wide parent of Dir() on unix.
func defaultBaseDir() string {
	if os.Geteuid() == 0 {
		return "/var/run"
	}
	// Check if /var/run is accessible/writable
	if err := syscall.Access("/var/run", 2); err == nil {
		return "/var/run"
	}
	if info, err := os.Stat("/var/run/vpngate"); err == nil && info.IsDir() {
		if err := syscall.Access("/var/run/vpngate", 2); err == nil {
			return "/var/run"
		}
	}
	// Fallback for non-root desktop GUI sessions
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return xdg
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "/var/run"
}
