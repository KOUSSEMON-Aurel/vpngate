//go:build !windows

package exec

import (
	"os/exec"
	"syscall"
)

// prepareProcGroup starts child processes in their own process group so the
// whole group (the child and any descendants it spawns, e.g. openvpn --up /
// --down helper scripts) can be torn down together on cancel.
func prepareProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcGroup kills the child's entire process group. Killing the group,
// rather than only the direct child, is required so that lingering
// descendants that inherited the stdout/stderr write-ends release them and
// the readers can reach EOF instead of blocking forever.
func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	// Fall back to killing the direct child if it is not group leader.
	_ = cmd.Process.Kill()
}
