package vpn

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// makeFreevpnBundle returns a minimal freevpn.me-style zip holding a single
// tcp443 OpenVPN config.
func makeFreevpnBundle(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("FreeVPN.me - Server1-NL/Server1-TCP443.ovpn")
	assert.NoError(t, err)
	_, err = w.Write([]byte("client\nproto tcp\nremote server1.freevpn.me 443\nauth-user-pass\ndev tun\n"))
	assert.NoError(t, err)
	assert.NoError(t, zw.Close())
	return buf.Bytes()
}

// freevpnPage returns a minimal freevpn.me accounts page carrying the
// shared credentials.
func freevpnPage() []byte {
	return []byte("**Username:** freevpn.me\n**Password:** k2YbR6Ve2JBe\n")
}

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
		switch r.URL.Path {
		case "/accounts":
			_, _ = w.Write(freevpnPage())
		case "/bundle.zip":
			_, _ = w.Write(makeFreevpnBundle(t))
		default:
			_, _ = w.Write(dat)
		}
	}))
	defer server.Close()

	originalVpnList := vpnList
	originalFreevpnBaseURL, originalFreevpnBundleURL := freevpnBaseURL, freevpnBundleURL
	vpnList = server.URL + "/vpnlist"
	freevpnBaseURL = server.URL + "/accounts"
	freevpnBundleURL = server.URL + "/bundle.zip"
	defer func() {
		vpnList = originalVpnList
		freevpnBaseURL, freevpnBundleURL = originalFreevpnBaseURL, originalFreevpnBundleURL
	}()

	servers, err := GetListWithOptions("", "", ListOptions{NoCache: true, DisableVpnbook: true})
	assert.NoError(t, err)
	assert.Equal(t, 100, len(*servers))
}

// TestGetListWithOptionsMergesVpnbook verifies vpnbook servers are appended
// to the vpngate list, each tagged with its source.
func TestGetListWithOptionsMergesVpnbook(t *testing.T) {
	dat, err := os.ReadFile("../../test_data/vpn_list.csv")
	assert.NoError(t, err)
	payload, err := os.ReadFile("../../test_data/vpnbook_rsc.txt")
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/vpnlist":
			_, _ = w.Write(dat)
		case "/accounts":
			_, _ = w.Write(freevpnPage())
		case "/bundle.zip":
			_, _ = w.Write(makeFreevpnBundle(t))
		default:
			_, _ = w.Write(payload)
		}
	}))
	defer server.Close()

	originalVpnList := vpnList
	originalVpnbookBaseURL, originalVpnbookConfigURL := vpnbookBaseURL, vpnbookConfigURL
	originalFreevpnBaseURL, originalFreevpnBundleURL := freevpnBaseURL, freevpnBundleURL
	vpnList = server.URL + "/vpnlist"
	vpnbookBaseURL = server.URL + "/page"
	vpnbookConfigURL = server.URL + "/config"
	freevpnBaseURL = server.URL + "/accounts"
	freevpnBundleURL = server.URL + "/bundle.zip"
	defer func() {
		vpnList = originalVpnList
		vpnbookBaseURL, vpnbookConfigURL = originalVpnbookBaseURL, originalVpnbookConfigURL
		freevpnBaseURL, freevpnBundleURL = originalFreevpnBaseURL, originalFreevpnBundleURL
	}()

	servers, err := GetListWithOptions("", "", ListOptions{NoCache: true})
	assert.NoError(t, err)
	assert.Equal(t, 110, len(*servers))

	var vpngateCount, vpnbookCount, freevpnCount, warpCount int
	for _, s := range *servers {
		switch s.Source {
		case SourceVpngate:
			vpngateCount++
		case SourceVpnbook:
			vpnbookCount++
		case SourceFreevpn:
			freevpnCount++
		case SourceWarp:
			warpCount++
		}
	}
	assert.Equal(t, 98, vpngateCount)
	assert.Equal(t, 10, vpnbookCount)
	assert.Equal(t, 1, freevpnCount)
	assert.Equal(t, 1, warpCount)
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
	originalFreevpnBaseURL := freevpnBaseURL
	vpnList = server.URL + "/vpnlist"
	vpnbookBaseURL = server.URL + "/page"
	freevpnBaseURL = server.URL + "/accounts"
	defer func() {
		vpnList = originalVpnList
		vpnbookBaseURL = originalVpnbookBaseURL
		freevpnBaseURL = originalFreevpnBaseURL
	}()

	servers, err := GetListWithOptions("", "", ListOptions{NoCache: true})
	assert.NoError(t, err)
	assert.Equal(t, 99, len(*servers))
	assert.Equal(t, SourceVpngate, (*servers)[0].Source)
	assert.Equal(t, SourceWarp, (*servers)[len(*servers)-1].Source)
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
