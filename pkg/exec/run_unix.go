//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
	"time"
)

// prepareProcGroup starts child processes in their own process group so the
// whole group (the child and any descendants it spawns, e.g. openvpn --up /
// --down helper scripts) can be torn down together on cancel.
func prepareProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcGroup tears down the child's entire process group. Killing the
// group, rather than only the direct child, is required so that lingering
// descendants that inherited the stdout/stderr write-ends release them and
// the readers can reach EOF instead of blocking forever.
//
// A SIGTERM is sent first so openvpn can tear down its tun interface and
// routes cleanly, and it is only escalated to SIGKILL after a short grace
// period.
func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	_ = cmd.Process.Signal(syscall.SIGTERM)

	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Direct child is gone; group members normally follow TERM.
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	// Fall back to killing the direct child if it is not group leader.
	_ = cmd.Process.Kill()
}
