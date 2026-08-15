package vpn

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseVpnbookServers parses a real vpnbook RSC payload fixture.
func TestParseVpnbookServers(t *testing.T) {
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	require.NoError(t, err)

	servers, err := parseVpnbookServers(payload)
	require.NoError(t, err)
	require.Len(t, servers, 10)

	assert.Equal(t, "us16", servers[0].ID)
	assert.Equal(t, "us16.vpnbook.com", servers[0].Hostname)
	assert.Equal(t, "147.135.15.16", servers[0].IPAddress)
	assert.Equal(t, "US", servers[0].CountryCode)
	assert.Equal(t, "United States", servers[0].CountryName)
}

// TestExtractVpnbookServersJSONMissingMarker rejects payloads without the
// server list.
func TestExtractVpnbookServersJSONMissingMarker(t *testing.T) {
	_, err := extractVpnbookServersJSON([]byte(`{"foo":"bar"}`))
	assert.Error(t, err)
}

// TestExtractVpnbookServersJSONUnbalanced rejects a truncated server array.
func TestExtractVpnbookServersJSONUnbalanced(t *testing.T) {
	_, err := extractVpnbookServersJSON([]byte(`"servers":[{"id":"us16"`))
	assert.Error(t, err)
}

// TestExtractVpnbookCredentials reads username and password from the fixture.
func TestExtractVpnbookCredentials(t *testing.T) {
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	require.NoError(t, err)

	creds := extractVpnbookCredentials(payload)
	assert.Equal(t, "vpnbook", creds.Username)
	assert.Equal(t, "ytw2awn", creds.Password)
	assert.False(t, creds.UpdatedAt.IsZero())
}

// TestFetchVpnbookServers fetches a server list and downloads each config.
func TestFetchVpnbookServers(t *testing.T) {
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	require.NoError(t, err)

	var (
		configCalls int32
		seenMu      sync.Mutex
		seen        = map[string]string{}
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/openvpn", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&configCalls, 1)
		seenMu.Lock()
		seen[r.URL.Query().Get("hostname")] = r.URL.Query().Get("protocol")
		seenMu.Unlock()
		_, _ = w.Write([]byte("client\nproto tcp\nremote us16.vpnbook.com 443\n"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	oldBaseURL, oldConfigURL := vpnbookBaseURL, vpnbookConfigURL
	vpnbookBaseURL, vpnbookConfigURL = server.URL+"/openvpn", server.URL+"/config"
	defer func() {
		vpnbookBaseURL, vpnbookConfigURL = oldBaseURL, oldConfigURL
	}()

	servers, err := FetchVpnbookServers(server.Client())
	require.NoError(t, err)
	require.Len(t, servers, 10)
	assert.Equal(t, int32(10), atomic.LoadInt32(&configCalls))

	// Every FQDN from the payload must have been requested over tcp443.
	for _, flightServer := range []vpnbookFlightServer{
		{ID: "us16", Hostname: "us16.vpnbook.com"},
		{ID: "ca196", Hostname: "ca196.vpnbook.com"},
	} {
		assert.Equal(t, "tcp443", seen[flightServer.Hostname], flightServer.Hostname)
	}

	assert.Equal(t, SourceVpnbook, servers[0].Source)
	assert.Equal(t, "us16", servers[0].HostName)
	assert.Equal(t, "United States", servers[0].CountryLong)
	assert.Equal(t, "US", servers[0].CountryShort)
	assert.Equal(t, "147.135.15.16", servers[0].IPAddr)

	config, err := base64.StdEncoding.DecodeString(servers[0].OpenVpnConfigData)
	require.NoError(t, err)
	assert.Contains(t, string(config), "remote us16.vpnbook.com 443")
	assert.Equal(t, "tcp", servers[0].Proto())
}

// TestFetchVpnbookServersSkipsFailedConfigs drops servers whose config cannot
// be fetched instead of failing the whole list.
func TestFetchVpnbookServersSkipsFailedConfigs(t *testing.T) {
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/openvpn", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/config", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	oldBaseURL, oldConfigURL := vpnbookBaseURL, vpnbookConfigURL
	vpnbookBaseURL, vpnbookConfigURL = server.URL+"/openvpn", server.URL+"/config"
	defer func() {
		vpnbookBaseURL, vpnbookConfigURL = oldBaseURL, oldConfigURL
	}()

	servers, err := FetchVpnbookServers(server.Client())
	require.NoError(t, err)
	assert.Empty(t, servers)
}

// TestFetchVpnbookCredentials scrapes credentials from the fixture endpoint.
func TestFetchVpnbookCredentials(t *testing.T) {
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1", r.Header.Get("RSC"))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	oldBaseURL := vpnbookBaseURL
	vpnbookBaseURL = server.URL
	defer func() { vpnbookBaseURL = oldBaseURL }()

	creds, err := FetchVpnbookCredentials(server.Client())
	require.NoError(t, err)
	assert.Equal(t, "vpnbook", creds.Username)
	assert.Equal(t, "ytw2awn", creds.Password)
}

// TestGetVpnbookCredentialsUsesCache verifies credentials are scraped once
// and served from cache afterwards.
func TestGetVpnbookCredentialsUsesCache(t *testing.T) {
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	require.NoError(t, err)

	var scrapes int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&scrapes, 1)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	oldBaseURL := vpnbookBaseURL
	vpnbookBaseURL = server.URL
	defer func() { vpnbookBaseURL = oldBaseURL }()

	t.Setenv("HOME", t.TempDir())

	first, err := GetVpnbookCredentials(server.Client())
	require.NoError(t, err)
	assert.Equal(t, "ytw2awn", first.Password)

	second, err := GetVpnbookCredentials(server.Client())
	require.NoError(t, err)
	assert.Equal(t, "ytw2awn", second.Password)

	assert.Equal(t, int32(1), atomic.LoadInt32(&scrapes))
}

// TestWriteVpnbookCredsFile writes OpenVPN's two-line auth-user-pass format.
func TestWriteVpnbookCredsFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path, err := WriteVpnbookCredsFile(VpnbookCredentials{
		Username: "vpnbook",
		Password: "secret",
	})
	require.NoError(t, err)
	assert.Equal(t, "vpnbook.creds", filepath.Base(path))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "vpnbook\nsecret\n", string(content))
}
