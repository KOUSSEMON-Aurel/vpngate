//go:build windows

package exec

import "os/exec"

// prepareProcGroup is a no-op on Windows where there is no process-group
// signal model; openvpn runs as a single foreground process here.
func prepareProcGroup(cmd *exec.Cmd) {}

// killProcGroup kills the direct child process.
func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
