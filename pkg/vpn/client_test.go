package vpn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectDetached(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ConnectDetached resolves an absolute openvpn.exe path on windows; not fakeable via PATH")
	}

	dir := t.TempDir()
	stub := filepath.Join(dir, "openvpn")
	argsFile := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\necho \"$@\" > \"" + argsFile + "\"\n"
	assert.NoError(t, os.WriteFile(stub, []byte(script), 0o755))

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var log bytes.Buffer
	configPath := filepath.Join(dir, "config.ovpn")
	cmd, err := ClientFor(Server{}).ConnectDetached(Server{}, configPath, "127.0.0.1:12345", &log, nil)
	assert.NoError(t, err)
	assert.NoError(t, cmd.Wait())

	args, err := os.ReadFile(argsFile)
	assert.NoError(t, err)
	assert.Contains(t, string(args), "--management 127.0.0.1 12345")
	assert.Contains(t, string(args), "--config "+configPath)
}

func TestConnectDetachedMissingExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ConnectDetached resolves an absolute openvpn.exe path on windows")
	}
	t.Setenv("PATH", t.TempDir())

	_, err := ClientFor(Server{}).ConnectDetached(Server{}, "config.ovpn", "127.0.0.1:12345", &bytes.Buffer{}, nil)
	assert.Error(t, err)
}

// TestClientForReturnsOpenVPNClient verifies every source resolves to a
// usable OpenVPN client.
func TestClientForReturnsOpenVPNClient(t *testing.T) {
	for _, source := range []string{"", SourceVpngate, SourceVpnbook} {
		client := ClientFor(Server{Source: source})
		assert.IsType(t, openVPNClient{}, client, "source %q", source)
	}
}

// TestOpenvpnArgsNoAuthForVpngate verifies vpngate configs are launched
// without an auth-user-pass argument: they carry inline credentials.
func TestOpenvpnArgsNoAuthForVpngate(t *testing.T) {
	args, err := openvpnArgs(Server{Source: SourceVpngate}, "/tmp/config.ovpn", 4)
	assert.NoError(t, err)
	assert.NotContains(t, args, "--auth-user-pass")
}

// TestOpenvpnArgsVpnbookAddsAuthUserPass verifies a vpnbook connect passes
// the shared credentials file explicitly so the bare "auth-user-pass"
// directive inside the config does not block on stdin.
func TestOpenvpnArgsVpnbookAddsAuthUserPass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cacheDir := filepath.Join(home, ".vpngate", "cache")
	assert.NoError(t, os.MkdirAll(cacheDir, 0o700))
	cacheData, err := json.Marshal(VpnbookCredentials{Username: "vpnbook", Password: "secret", UpdatedAt: time.Now()})
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filepath.Join(cacheDir, vpnbookCredsCacheFile), cacheData, 0o600))

	args, err := openvpnArgs(Server{Source: SourceVpnbook}, "/tmp/config.ovpn", 4)
	assert.NoError(t, err)

	authIdx := -1
	for i, arg := range args {
		if arg == "--auth-user-pass" {
			authIdx = i
			break
		}
	}
	assert.GreaterOrEqual(t, authIdx, 0, "expected --auth-user-pass in args: %v", args)
	assert.LessOrEqual(t, authIdx+1, len(args)-1)

	credsFile := args[authIdx+1]
	content, err := os.ReadFile(credsFile)
	assert.NoError(t, err)
	assert.Equal(t, "vpnbook\nsecret\n", string(content))
	assert.NotContains(t, args, "--management")
}

// TestWriteServerConfigRoundTrip verifies WriteServerConfig writes the
// decoded config to a temp file that is cleaned up by the caller.
func TestWriteServerConfigRoundTrip(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	config := "client\ndev tun\nremote 10.0.0.1 1194\n"
	server := Server{
		HostName:          "test",
		OpenVpnConfigData: base64.StdEncoding.EncodeToString([]byte(config)),
		Source:            SourceVpngate,
	}

	path, err := WriteServerConfig(server)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(filepath.Base(path), "vpngate-openvpn-config-"))

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	content := string(got)
	assert.Contains(t, content, "remote 10.0.0.1 1194")

	// The IPv6 black-hole must match the host's reality, same as connect.
	if HostRoutesIPv6() {
		assert.Contains(t, content, "route-ipv6 ::/0")
	} else {
		assert.NotContains(t, content, "route-ipv6")
	}

	assert.NoError(t, os.Remove(path))
}

// TestServerConfigMissingData verifies an empty config is rejected before
// reaching openvpn.
func TestServerConfigMissingData(t *testing.T) {
	_, err := ServerConfig(Server{})
	assert.Error(t, err)
}
