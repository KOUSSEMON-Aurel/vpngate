package vpn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWgcfEnsureProfileAcceptTos verifies wgcf register is invoked
// non-interactively with --accept-tos so first-run provisioning never
// blocks on Cloudflare's ToS prompt.
func TestWgcfEnsureProfileAcceptTos(t *testing.T) {
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
