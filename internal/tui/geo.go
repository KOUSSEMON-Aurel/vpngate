package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// geoInfo is the user's public location, resolved once at TUI startup.
type geoInfo struct {
	loaded bool
	code   string // ISO 3166-1 alpha-2, e.g. "FR"
	name   string // full country name, e.g. "France"
	region string
	city   string
	err    error
}

// geoMsg carries the result of the async geolocation lookup.
type geoMsg geoInfo

// geoCmd resolves the user's public location without blocking the TUI.
func geoCmd() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 4 * time.Second}
		req, err := http.NewRequest(http.MethodGet,
			"https://ipwho.is/", nil)
		if err != nil {
			return geoMsg{loaded: true, err: err}
		}
		req.Header.Set("User-Agent", "vpngate-tui/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return geoMsg{loaded: true, err: err}
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return geoMsg{loaded: true, err: fmt.Errorf("geo lookup: http %d", resp.StatusCode)}
		}
		var data struct {
			Success     bool   `json:"success"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
			Region      string `json:"region"`
			City        string `json:"city"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return geoMsg{loaded: true, err: err}
		}
		if !data.Success || data.CountryCode == "" {
			return geoMsg{loaded: true, err: fmt.Errorf("geo lookup: no location")}
		}
		return geoMsg{loaded: true, code: data.CountryCode, name: data.Country,
			region: data.Region, city: data.City}
	}
}

// locLabel renders the human-readable location for the status bar and footer.
func (g geoInfo) locLabel() string {
	if !g.loaded {
		return "locating…"
	}
	if g.err != nil || g.code == "" {
		return "unknown"
	}
	parts := []string{g.name}
	if g.city != "" {
		parts = append(parts, g.city)
	}
	flag := countryFlag(g.code)
	label := strings.Join(parts, ", ")
	if flag != "" {
		label = flag + " " + label
	}
	return label
}

// countryCoords maps ISO 3166-1 alpha-2 codes to (longitude, latitude) so the
// world map can pin the user's approximate position.
var countryCoords = map[string][2]float64{
	"JP": {138.3, 36.2}, "US": {-98.0, 39.0}, "CA": {-106.0, 56.0},
	"MX": {-102.5, 23.6}, "BR": {-48.0, -10.0}, "AR": {-63.6, -34.0},
	"CL": {-71.5, -33.0}, "PE": {-75.0, -9.2}, "CO": {-74.3, 4.6},
	"VE": {-66.0, 8.0}, "GB": {-2.0, 54.0}, "IE": {-8.2, 53.4},
	"FR": {2.2, 46.2}, "DE": {10.4, 51.2}, "NL": {5.3, 52.1},
	"BE": {4.5, 50.5}, "CH": {8.2, 46.8}, "AT": {14.5, 47.5},
	"IT": {12.6, 42.8}, "ES": {-3.7, 40.4}, "PT": {-8.2, 39.6},
	"SE": {15.0, 62.0}, "NO": {8.5, 61.5}, "FI": {26.0, 64.0},
	"DK": {9.5, 56.3}, "PL": {19.1, 51.9}, "CZ": {15.5, 49.8},
	"SK": {19.5, 48.7}, "HU": {19.5, 47.2}, "RO": {25.0, 45.9},
	"BG": {25.0, 42.7}, "GR": {22.0, 39.1}, "TR": {35.0, 39.0},
	"UA": {31.2, 49.0}, "RU": {100.0, 60.0}, "EG": {30.0, 26.8},
	"IL": {34.9, 31.4}, "SA": {45.0, 24.0}, "AE": {54.3, 23.4},
	"IN": {78.0, 21.0}, "PK": {69.0, 30.0}, "CN": {104.0, 35.0},
	"KR": {127.8, 36.5}, "TW": {121.0, 23.7}, "HK": {114.2, 22.3},
	"SG": {103.8, 1.4}, "MY": {102.0, 4.0}, "TH": {100.9, 15.9},
	"VN": {107.8, 16.5}, "PH": {122.0, 12.9}, "ID": {113.9, -0.8},
	"NZ": {172.8, -41.5}, "AU": {134.0, -25.0}, "ZA": {25.0, -29.0},
	"NG": {8.0, 9.6}, "KE": {37.9, 0.0}, "MA": {-7.1, 31.8},
	"TN": {9.6, 34.0},
}
