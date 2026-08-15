package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
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
	backend, err := detectWarpBackend()
	assert.Equal(t, warpBackendNone, backend)
	assert.Error(t, err)
}

// TestDetectWarpBackendWgcf verifies wgcf is preferred when both wgcf and
// wg-quick are available.
func TestDetectWarpBackendWgcf(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "wgcf")
	writeFakeBin(t, bin, "wg-quick")
	t.Setenv("PATH", bin)

	backend, err := detectWarpBackend()
	assert.Equal(t, warpBackendWgcf, backend)
	assert.NoError(t, err)
}

// TestDetectWarpBackendWgcfWithoutWgQuick verifies wgcf alone is not
// enough to select the wgcf backend (it needs wg-quick to bring the
// tunnel up).
func TestDetectWarpBackendWgcfWithoutWgQuick(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "wgcf")
	t.Setenv("PATH", bin)

	backend, err := detectWarpBackend()
	assert.Equal(t, warpBackendNone, backend)
	assert.Error(t, err)
}

// TestDetectWarpBackendCli verifies warp-cli is used as the fallback when
// the wgcf tool chain is missing.
func TestDetectWarpBackendCli(t *testing.T) {
	bin := t.TempDir()
	writeFakeBin(t, bin, "warp-cli")
	t.Setenv("PATH", bin)

	backend, err := detectWarpBackend()
	assert.Equal(t, warpBackendCli, backend)
	assert.NoError(t, err)
}

// TestWarpWgcfProfileFlag verifies --wgcf-config wins over the default
// profile path.
func TestWarpWgcfProfileFlag(t *testing.T) {
	flagWgcfConfig = "/tmp/custom-wgcf.conf"
	defer func() { flagWgcfConfig = "" }()

	profile, err := warpWgcfProfile()
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/custom-wgcf.conf", profile)
}

// TestWarpWgcfEnsureProfileDefault verifies the default profile path lives under
// $XDG_CONFIG_HOME/wgcf on Linux.
func TestWarpWgcfProfileDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME only drives os.UserConfigDir on Linux")
	}
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	profile, err := warpWgcfProfile()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(configHome, "wgcf", "wgcf-profile.conf"), profile)
}

// TestWarpWgcfEnsureProfileAcceptTos verifies wgcf register is invoked
// non-interactively with --accept-tos so first-run provisioning never
// blocks on Cloudflare's ToS prompt.
func TestWarpWgcfEnsureProfileAcceptTos(t *testing.T) {
	bin := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "wgcf-args")
	script := "#!/bin/sh\necho \"$@\" >> " + argsFile + "\nexit 0\n"
	assert.NoError(t, os.WriteFile(filepath.Join(bin, "wgcf"), []byte(script), 0o755))
	t.Setenv("PATH", bin)

	profileDir := t.TempDir()
	assert.NoError(t, wgcfEnsureProfile(profileDir))

	args, err := os.ReadFile(argsFile)
	assert.NoError(t, err)
	assert.Equal(t, "register --accept-tos\ngenerate\n", string(args))
}
