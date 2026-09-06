package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/davegallant/vpngate/pkg/daemon"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// serveFixture returns a static server list covering both sources, both
// protos and a vpnbook transport, for the serve API tests.
func serveFixture() []vpn.Server {
	udpConfig := base64.StdEncoding.EncodeToString([]byte("proto udp\nremote 203.0.113.1 1194\n"))
	tcpConfig := base64.StdEncoding.EncodeToString([]byte("proto tcp\nremote 203.0.113.2 443\n"))
	return []vpn.Server{
		{HostName: "jp-udp", CountryLong: "Japan", CountryShort: "JP", Score: 90, IPAddr: "203.0.113.10", Ping: "100", OpenVpnConfigData: udpConfig, Source: vpn.SourceVpngate},
		{HostName: "fr-tcp", CountryLong: "France", CountryShort: "FR", Score: 70, IPAddr: "203.0.113.20", Ping: "40", OpenVpnConfigData: tcpConfig, Source: vpn.SourceVpngate},
		{HostName: "vb-tcp443", CountryLong: "United States", CountryShort: "US", Score: 80, IPAddr: "203.0.113.30", Ping: "60", OpenVpnConfigData: tcpConfig, Source: vpn.SourceVpnbook, Transport: "tcp443"},
		{HostName: "vb-udp53", CountryLong: "United States", CountryShort: "US", Score: 75, IPAddr: "203.0.113.31", Ping: "60", OpenVpnConfigData: udpConfig, Source: vpn.SourceVpnbook, Transport: "udp53"},
	}
}

// newTestServeAPI wires a serveAPI whose server list is the fixture,
// keeping the tests off the network.
func newTestServeAPI(t *testing.T) *serveAPI {
	t.Helper()
	t.Setenv(daemon.DirEnvVar, t.TempDir())
	return &serveAPI{
		fetchServers: func(vpn.ListOptions) ([]vpn.Server, error) {
			return serveFixture(), nil
		},
	}
}

func doRequest(api *serveAPI, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if body != "" {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	}
	rec := httptest.NewRecorder()
	api.handler().ServeHTTP(rec, req)
	return rec
}

// TestServeHealth verifies the health endpoint answers and CORS is wide
// open for the future mobile frontend.
func TestServeHealth(t *testing.T) {
	api := newTestServeAPI(t)
	rec := doRequest(api, http.MethodGet, "/api/health", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	var body map[string]bool
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body["ok"])
}

// TestServeServersFilters verifies the server list endpoint applies each
// filter parameter and never leaks the embedded OpenVPN config.
func TestServeServersFilters(t *testing.T) {
	api := newTestServeAPI(t)

	rec := doRequest(api, http.MethodGet, "/api/servers", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var all []map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &all))
	assert.Len(t, all, 4)
	assert.Contains(t, all[0], "hostname")
	assert.Contains(t, all[0], "country_long")
	assert.NotContains(t, all[0], "OpenVpnConfigData")

	cases := []struct {
		query string
		want  int
	}{
		{"?country=us", 2},
		{"?proto=udp", 2},
		{"?source=vpnbook", 2},
		{"?source=vpnbook&transport=tcp443", 1},
		{"?min_score=80", 2},
		{"?max_ping=60", 3},
		{"?country=zz", 0},
	}
	for _, c := range cases {
		rec := doRequest(api, http.MethodGet, "/api/servers"+c.query, "")
		assert.Equal(t, http.StatusOK, rec.Code, c.query)
		var got []map[string]any
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got), c.query)
		assert.Len(t, got, c.want, c.query)
	}
}

// TestServeConnectDisconnectLifecycle verifies connect/status/disconnect
// over HTTP: one connection at a time, and the state returns to
// DISCONNECTED after a stop.
func TestServeConnectDisconnectLifecycle(t *testing.T) {
	api := newTestServeAPI(t)

	rec := doRequest(api, http.MethodPost, "/api/connect", `{"hostname":"jp-udp"}`)
	assert.Equal(t, http.StatusAccepted, rec.Code)

	rec = doRequest(api, http.MethodPost, "/api/connect", `{"hostname":"fr-tcp"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = doRequest(api, http.MethodGet, "/api/status", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var status statusResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	assert.Equal(t, "jp-udp", status.HostName)

	rec = doRequest(api, http.MethodPost, "/api/disconnect", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var stop map[string]string
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stop))
	assert.Equal(t, "DISCONNECTED", stop["state"])

	rec = doRequest(api, http.MethodGet, "/api/status", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	assert.Equal(t, "DISCONNECTED", status.State)
}

// TestServeConnectErrors verifies the failure modes: unknown hostname,
// filters matching nothing, and disconnect while idle.
func TestServeConnectErrors(t *testing.T) {
	api := newTestServeAPI(t)

	rec := doRequest(api, http.MethodPost, "/api/connect", `{"hostname":"nope"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doRequest(api, http.MethodPost, "/api/connect", `{"country":"zz"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doRequest(api, http.MethodPost, "/api/connect", `not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doRequest(api, http.MethodPost, "/api/disconnect", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestServeConnectProtocolNormalization verifies that passing proto ("udp" or "tcp")
// or protocol ("openvpn") is correctly handled without rejecting the connection.
func TestServeConnectProtocolNormalization(t *testing.T) {
	api := newTestServeAPI(t)

	// Sending protocol: "udp" should normalize and succeed
	rec := doRequest(api, http.MethodPost, "/api/connect", `{"hostname":"jp-udp","protocol":"udp"}`)
	assert.Equal(t, http.StatusAccepted, rec.Code)

	_ = doRequest(api, http.MethodPost, "/api/disconnect", "")

	// Sending explicit proto and protocol should succeed
	rec = doRequest(api, http.MethodPost, "/api/connect", `{"hostname":"fr-tcp","proto":"tcp","protocol":"openvpn"}`)
	assert.Equal(t, http.StatusAccepted, rec.Code)

	_ = doRequest(api, http.MethodPost, "/api/disconnect", "")
}

// TestServeStatusReportsError verifies that lastError is carried in /api/status.
func TestServeStatusReportsError(t *testing.T) {
	api := newTestServeAPI(t)
	api.lastError = "relay refused connection (AUTH_FAILED)"

	rec := doRequest(api, http.MethodGet, "/api/status", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	var status statusResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	assert.Equal(t, "DISCONNECTED", status.State)
	assert.Equal(t, "relay refused connection (AUTH_FAILED)", status.Error)
}
