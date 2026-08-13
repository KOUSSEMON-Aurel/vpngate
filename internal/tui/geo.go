package tui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
			"https://ip-api.com/json/?fields=status,country,countryCode,regionName,city", nil)
		if err != nil {
			return geoMsg{loaded: true, err: err}
		}
		req.Header.Set("User-Agent", "vpngate-tui/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return geoMsg{loaded: true, err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return geoMsg{loaded: true, err: fmt.Errorf("geo lookup: http %d", resp.StatusCode)}
		}
		var data struct {
			Status      string `json:"status"`
			Country     string `json:"country"`
			CountryCode string `json:"countryCode"`
			RegionName  string `json:"regionName"`
			City        string `json:"city"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return geoMsg{loaded: true, err: err}
		}
		if data.Status != "success" || data.CountryCode == "" {
			return geoMsg{loaded: true, err: fmt.Errorf("geo lookup: no location")}
		}
		return geoMsg{loaded: true, code: data.CountryCode, name: data.Country,
			region: data.RegionName, city: data.City}
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

// worldMap is a coarse equirectangular world map, one string per latitude
// band. "░" is ocean, "▓" is land. Every row is exactly 26 runes wide and the
// lat field is the band's centre used to place the location marker.
var worldMap = []struct {
	lat  float64
	land string
}{
	{87, "░░░░░░░░░░░░░░░░░░░░░░░░░░"},
	{58, "░▓▓▓▓░░░░░░░░░░░░▓▓▓▓▓▓▓"},
	{42, "░░▓░▓▓▓▓▓░░░▓▓▓▓▓▓▓▓▓▓▓▓▓"},
	{27, "░░░░░▓▓▓▓░░░▓▓▓▓▓▓▓▓▓▓▓▓░░"},
	{12, "░░░░░░░▓▓▓▓▓▓▓▓▓▓▓▓░▓▓▓▓░░"},
	{-2, "░░░░░░░░▓▓▓▓░▓▓▓▓▓▓░░▓▓▓░░"},
	{-17, "░░░░░░░░░▓▓▓░░▓░▓░░░░▓▓▓▓░"},
	{-32, "░░░░░░░░▓▓░░░░░░░░░░░░▓▓░░"},
	{-47, "░░░░░░░░▓░░░░░░░░░░░░░░░░░"},
	{-62, "░░░░░░░░░░░░░░░░░░░░░░░░░░"},
	{-77, "░░░░░░░░░░░░░░░░░░░░░░░░░░"},
}

// mapWidth is the number of columns in each worldMap row.
const mapWidth = 26

// mapCoord approximates the screen coordinates of a country code. The world
// map is hand-drawn so the marker is a best-effort pin, not a survey point.
func mapCoord(code string) (row, col int, ok bool) {
	c, ok := countryCoords[strings.ToUpper(code)]
	if !ok {
		return 0, 0, false
	}
	lat, lon := c[1], c[0]
	row = 0
	for i := range worldMap {
		if math.Abs(lat-worldMap[i].lat) < math.Abs(lat-worldMap[row].lat) {
			row = i
		}
	}
	col = int(math.Round((lon + 180) / 360 * (mapWidth - 1)))
	if col < 0 {
		col = 0
	}
	if col > mapWidth-1 {
		col = mapWidth - 1
	}
	return row, col, true
}

// mapView is a pre-rendered location panel for the right-hand column.
type mapView struct {
	lines []string
	width int
}

// mapView builds the pixel world map with the user's location pinned. It
// returns nil when the terminal is too narrow or short to fit it.
func (m *model) mapView() *mapView {
	const width = 28
	rows := m.visibleRows()
	if m.width < 120 || rows < 1 {
		return nil
	}

	var lines []string
	title := styleGeo.Render("YOU")
	sub := ""
	if m.geo.loaded && m.geo.err == nil && m.geo.code != "" {
		title = styleGeo.Render("YOU") + " " + countryFlag(m.geo.code)
		sub = m.geo.code
		if m.geo.city != "" {
			sub += " · " + m.geo.city
		} else if m.geo.name != "" {
			sub += " · " + m.geo.name
		}
	} else if !m.geo.loaded {
		sub = "locating…"
	} else {
		sub = "unavailable"
	}

	lines = append(lines, " "+title)
	markerRow, markerCol, hasMarker := mapCoord(m.geo.code)
	for i, band := range worldMap {
		var b strings.Builder
		b.WriteByte(' ')
		runes := []rune(band.land)
		for c := 0; c < len(runes); c++ {
			switch {
			case hasMarker && i == markerRow && c == markerCol:
				b.WriteString(styleMarker.Render("●"))
			case runes[c] == '▓':
				b.WriteString(styleLand.Render("▓"))
			default:
				b.WriteString(styleOcean.Render("░"))
			}
		}
		lines = append(lines, b.String())
	}
	lines = append(lines, " "+styleDim.Render(truncate(sub, width-2)))

	pad := lipgloss.NewStyle()
	for i, l := range lines {
		lines[i] = pad.Width(width).Render(truncate(l, width))
	}
	return &mapView{lines: lines, width: width}
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
