package vpn

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerProto(t *testing.T) {
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

	tests := []struct {
		name string
		conf string
		want string
	}{
		{"tcp", "client\ndev tun\nproto tcp\nremote 1.2.3.4 1458", "tcp"},
		{"tcp-client", "client\nproto tcp-client\nremote 1.2.3.4 443", "tcp"},
		{"udp", "client\nproto udp\nremote 1.2.3.4 1194", "udp"},
		{"commented proto line only", "client\n# proto tcp\nproto udp\n", "udp"},
		{"no proto line", "client\nremote 1.2.3.4 1194", ""},
		{"empty config", "", ""},
		{"invalid base64", "!!!not-base64!!!", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configData string
			if tt.conf != "invalid base64" {
				configData = enc(tt.conf)
			} else {
				configData = tt.conf
			}
			s := Server{OpenVpnConfigData: configData}
			assert.Equal(t, tt.want, s.Proto())
		})
	}
}

// TestGetListWithOptions fetches and parses a local fixture served over
// HTTP, exercising the same code path as a real fetch without depending on
// vpngate.net being reachable.
func TestGetListWithOptions(t *testing.T) {
	dat, err := os.ReadFile("../../test_data/vpn_list.csv")
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(dat)
	}))
	defer server.Close()

	originalVpnList := vpnList
	vpnList = server.URL
	defer func() { vpnList = originalVpnList }()

	servers, err := GetListWithOptions("", "", ListOptions{NoCache: true, DisableVpnbook: true})
	assert.NoError(t, err)
	assert.Equal(t, 98, len(*servers))
}

// TestGetListWithOptionsMergesVpnbook verifies vpnbook servers are appended
// to the vpngate list, each tagged with its source.
func TestGetListWithOptionsMergesVpnbook(t *testing.T) {
	dat, err := os.ReadFile("../../test_data/vpn_list.csv")
	assert.NoError(t, err)
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/vpnlist" {
			_, _ = w.Write(dat)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	originalVpnList := vpnList
	originalVpnbookBaseURL, originalVpnbookConfigURL := vpnbookBaseURL, vpnbookConfigURL
	vpnList = server.URL + "/vpnlist"
	vpnbookBaseURL = server.URL + "/page"
	vpnbookConfigURL = server.URL + "/config"
	defer func() {
		vpnList = originalVpnList
		vpnbookBaseURL, vpnbookConfigURL = originalVpnbookBaseURL, originalVpnbookConfigURL
	}()

	servers, err := GetListWithOptions("", "", ListOptions{NoCache: true})
	assert.NoError(t, err)
	assert.Equal(t, 108, len(*servers))

	var vpngateCount, vpnbookCount int
	for _, s := range *servers {
		switch s.Source {
		case SourceVpngate:
			vpngateCount++
		case SourceVpnbook:
			vpnbookCount++
		}
	}
	assert.Equal(t, 98, vpngateCount)
	assert.Equal(t, 10, vpnbookCount)
}

// TestGetListWithOptionsVpnbookFailureKeepsVpngate verifies a vpnbook fetch
// failure degrades to the vpngate list instead of failing the whole request.
func TestGetListWithOptionsVpnbookFailureKeepsVpngate(t *testing.T) {
	dat, err := os.ReadFile("../../test_data/vpn_list.csv")
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/vpnlist" {
			_, _ = w.Write(dat)
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	originalVpnList := vpnList
	originalVpnbookBaseURL := vpnbookBaseURL
	vpnList = server.URL + "/vpnlist"
	vpnbookBaseURL = server.URL + "/page"
	defer func() {
		vpnList = originalVpnList
		vpnbookBaseURL = originalVpnbookBaseURL
	}()

	servers, err := GetListWithOptions("", "", ListOptions{NoCache: true})
	assert.NoError(t, err)
	assert.Equal(t, 98, len(*servers))
	assert.Equal(t, SourceVpngate, (*servers)[0].Source)
}

// TestParseVpnList parses a local copy of vpn list csv
func TestParseVpnList(t *testing.T) {
	dat, err := os.Open("../../test_data/vpn_list.csv")
	assert.NoError(t, err)

	servers, err := parseVpnList(dat)
	assert.NoError(t, err)

	assert.Equal(t, len(*servers), 98)

	assert.Equal(t, (*servers)[0].CountryLong, "Japan")
	assert.Equal(t, (*servers)[0].CountryShort, "jp")
	assert.Equal(t, (*servers)[0].HostName, "public-vpn-227")
	assert.Equal(t, (*servers)[0].Ping, "13")
	assert.Equal(t, (*servers)[0].Score, 2086924)

	assert.Equal(t, int64(3399364), (*servers)[0].Speed)
	assert.Equal(t, 568, (*servers)[0].NumVpnSessions)
	assert.Equal(t, int64(23293814598), (*servers)[0].Uptime)
	assert.Equal(t, int64(3088324), (*servers)[0].TotalUsers)
	assert.Equal(t, int64(109318687634581), (*servers)[0].TotalTraffic)
	assert.Equal(t, "2weeks", (*servers)[0].LogType)
	assert.Equal(t, "Daiyuu Nobori_ Japan. Academic Use Only.", (*servers)[0].Operator)
	assert.Equal(t, "", (*servers)[0].Message)

	for _, s := range *servers {
		assert.NotEqual(t, "Korea Republic of", s.CountryLong, "CountryLong should be aliased to South Korea")
	}
}
