//go:build !windows

package tui

import (
	"os"
	osexec "os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// detectPrivilege determines how the current process can create a tun
// interface. It is called once at TUI startup; the result does not change
// during the session (setuid is resolved at exec time).
func detectPrivilege() privState {
	if os.Geteuid() == 0 {
		return privRoot
	}
	if processHasCapNetAdmin() || openvpnHasCapNetAdmin() {
		return privCapNetAdmin
	}
	return privNone
}

// processHasCapNetAdmin reads the effective capability set of the current
// process from /proc/self/status. This covers a vpngate binary that was
// itself granted CAP_NET_ADMIN via setcap.
func processHasCapNetAdmin() bool {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "CapEff:")), 16, 64)
		return err == nil && v&capNetAdmin != 0
	}
	return false
}

// openvpnHasCapNetAdmin reports whether the openvpn binary on PATH carries
// the CAP_NET_ADMIN file capability (i.e. "sudo setcap cap_net_admin+ep
// /usr/bin/openvpn" was run). It reads the security.capability xattr of the
// resolved binary.
func openvpnHasCapNetAdmin() bool {
	path, err := osexec.LookPath("openvpn")
	if err != nil {
		return false
	}
	// vfs_cap_data layout: magic_etc (4 bytes LE) followed by permitted
	// and inheritable capability sets (3 x 32-bit words each).
	buf := make([]byte, 20)
	n, err := unix.Getxattr(path, "security.capability", buf)
	if err != nil {
		return false
	}
	if n < 8 {
		return false
	}
	permitted := uint64(buf[4]) | uint64(buf[5])<<8 | uint64(buf[6])<<16 | uint64(buf[7])<<24
	return permitted&capNetAdmin != 0
}
