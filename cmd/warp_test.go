package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/davegallant/vpngate/pkg/vpn"
)

// writeFakeBin creates an executable stub named name inside dir so it can
// be picked up by exec.LookPath via PATH.
func writeFakeBin(t *testing.T, dir, name string) {
	t.Helper()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755))
}

// TestDetectWarpBackendNone verifies that without wgcf, wg-quick or
// warp-cli on PATH no WARP backend is selected.
func TestDetectWarpBackendNone(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	backend, err := vpn.DetectWarpBackend()
	assert.Equal(t, vpn.WarpBackendNone, backend)
	assert.Error(t, err)
}

// TestDetectWarpBackendWgcf verifies wgcf is preferred when both wgcf and
// wg-quick are available.
func TestDetectWarpBackendWgcf(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "wgcf")
	writeFakeBin(t, bin, "wg-quick")
	t.Setenv("PATH", bin)

	backend, err := vpn.DetectWarpBackend()
	assert.Equal(t, vpn.WarpBackendWgcf, backend)
	assert.NoError(t, err)
}

// TestDetectWarpBackendWgcfWithoutWgQuick verifies wgcf alone is not
// enough to select the wgcf backend (it needs wg-quick to bring the
// tunnel up).
func TestDetectWarpBackendWgcfWithoutWgQuick(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "wgcf")
	t.Setenv("PATH", bin)

	backend, err := vpn.DetectWarpBackend()
	assert.Equal(t, vpn.WarpBackendNone, backend)
	assert.Error(t, err)
}

// TestDetectWarpBackendCli verifies warp-cli is used as the fallback when
// the wgcf tool chain is missing.
func TestDetectWarpBackendCli(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "warp-cli")
	t.Setenv("PATH", bin)

	backend, err := vpn.DetectWarpBackend()
	assert.Equal(t, vpn.WarpBackendCli, backend)
	assert.NoError(t, err)
}

// TestWarpWgcfProfileOverride verifies an explicit profile path wins over
// the default.
func TestWarpWgcfProfileOverride(t *testing.T) {
	profile, err := vpn.WarpWgcfProfile("/tmp/custom-wgcf.conf")
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/custom-wgcf.conf", profile)
}

// TestWarpWgcfProfileDefault verifies the default profile path lives under
// $XDG_CONFIG_HOME/wgcf on Linux.
func TestWarpWgcfProfileDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME only drives os.UserConfigDir on Linux")
	}
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	profile, err := vpn.WarpWgcfProfile("")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(configHome, "wgcf", "wgcf-profile.conf"), profile)
}
