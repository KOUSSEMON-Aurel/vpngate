package vpn

import (
	"context"
	"fmt"
	"io"
	"net"
	osexec "os/exec"
	"runtime"
	"strconv"
	"syscall"

	"github.com/davegallant/vpngate/pkg/exec"
)

// executablePath returns the platform-specific path to the openvpn
// binary.
func executablePath() string {
	if runtime.GOOS == "windows" {
		return `C:\Program Files\OpenVPN\bin\openvpn.exe`
	}
	return "openvpn"
}

// Connect to a specified OpenVPN configuration. Blocks until openvpn
// exits, streaming its output through pkg/exec's logger.
func Connect(configPath string) error {
	return exec.Run(executablePath(), ".", "--verb", "4", "--config", configPath, "--data-ciphers", "AES-128-CBC")
}

// ConnectContext is Connect with a cancelable context and an optional
// output stream: when ctx is canceled openvpn is killed, and each output
// line is written to out (when non-nil) as it is produced.
func ConnectContext(ctx context.Context, configPath string, out io.Writer) error {
	return ConnectContextWithVerb(ctx, configPath, out, 4)
}

// ConnectContextWithVerb is ConnectContext with an explicit openvpn
// verbosity level.
func ConnectContextWithVerb(ctx context.Context, configPath string, out io.Writer, verb int) error {
	return exec.RunContext(ctx, executablePath(), ".", out, "--verb", strconv.Itoa(verb), "--config", configPath, "--data-ciphers", "AES-128-CBC")
}

// ConnectDetached starts openvpn with a management interface enabled at
// managementAddr, detached via sysProcAttr so it outlives the calling
// process, writing its combined stdout/stderr to logWriter. It returns as
// soon as the process has started; callers wait on the returned *exec.Cmd
// independently (via cmd.Wait()) to learn when it exits.
func ConnectDetached(configPath, managementAddr string, logWriter io.Writer, sysProcAttr *syscall.SysProcAttr) (*osexec.Cmd, error) {
	executable := executablePath()
	if _, err := osexec.LookPath(executable); err != nil {
		return nil, fmt.Errorf("%s is required, please install it", executable)
	}

	host, port, err := net.SplitHostPort(managementAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid management address %q: %w", managementAddr, err)
	}

	cmd := osexec.Command(
		executable,
		"--verb", "4",
		"--config", configPath,
		"--data-ciphers", "AES-128-CBC",
		"--management", host, port,
	)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	cmd.SysProcAttr = sysProcAttr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
