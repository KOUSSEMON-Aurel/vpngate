package tui

import (
	"encoding/base64"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// landRaster lazily decodes the base64 land mask emitted by tools/genland.
var landBits []byte

// landAt reports whether the 110m land mask marks (lat, lon) as land. lon is
// in [-180, 180], lat in [-90, 90].
func landAt(lat, lon float64) bool {
	if landBits == nil {
		landBits, _ = base64.StdEncoding.DecodeString(landRasterEncoded)
	}
	if landBits == nil {
		return false
	}
	lat = math.Max(-90, math.Min(90, lat))
	lon = math.Mod(lon, 360)
	if lon > 180 {
		lon -= 360
	}
	if lon < -180 {
		lon += 360
	}
	j := int(math.Floor(90 - lat))
	if j < 0 {
		j = 0
	}
	if j > 179 {
		j = 179
	}
	i := int(math.Floor(lon + 180))
	i = ((i % 360) + 360) % 360
	bit := j*360 + i
	return landBits[bit/8]&(1<<(7-uint(bit%8))) != 0
}

// globeRamps are the greyscale ramps used to fake a single light source.
// Index 0 is the darkest (terminator), index 3 the brightest (sun side).
// The globe itself stays monochrome; only the location pins carry colour.
var globeRamps = struct {
	land  [4]lipgloss.Style
	ocean [4]lipgloss.Style
}{
	land: [4]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("254")),
	},
	ocean: [4]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("234")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	},
}

// globeView is a pre-rendered rotating globe panel for the right-hand column.
type globeView struct {
	lines []string
	width int
}

// sunDir is the light direction in view space (x right, y up, z towards the
// viewer): upper-left-front, so the lit side tilts towards the viewer and the
// terminator wraps to a dark rim on the bottom-right.
var sunDir = [3]float64{-0.353, 0.505, 0.787}

// shadeLevel maps a diffuse factor in [0,1] to a colour ramp index.
func shadeLevel(diffuse float64) int {
	l := int(diffuse * 3.99)
	if l < 0 {
		return 0
	}
	if l > 3 {
		return 3
	}
	return l
}

// globeRadius computes the globe's disc radius from the current terminal
// size. The radius is bounded by both axes: the height drives how many disc
// rows fit next to the list body, the width bounds how many columns the right
// column may take while the list keeps at least minListW columns. The globe
// therefore grows when the terminal grows in either direction and shrinks
// back when the terminal is squeezed.
func (m *model) globeRadius() int {
	rows := m.visibleRows()
	if rows < 7 {
		return 0
	}
	r := (rows + 2) / 2
	if m.height < 16 {
		r = (rows - 1) / 2
	}
	if r > 14 {
		r = 14
	}
	if rw := (m.width - minListW) / 2; r > rw {
		r = rw
	}
	if r < 4 {
		return 0
	}
	return r
}

// globeView builds the orthographically projected, sun-shaded globe with the
// user's location pinned. It returns nil when the terminal is too narrow or
// short to fit it. The rendered result is cached and only rebuilt when the
// terminal size, rotation or geo info changed, so resize bursts and spinner
// ticks do not redo the per-pixel work every frame.
func (m *model) globeView() *globeView {
	if !m.globeDirty && m.globeCache != nil {
		return m.globeCache
	}
	m.globeDirty = false
	r := m.globeRadius()
	if r == 0 {
		return nil
	}
	width := 2*r + 3

	lon0 := float64(m.globeRot % 360)
	phi0, lam0 := 15*math.Pi/180, lon0*math.Pi/180
	sinP0, cosP0 := math.Sin(phi0), math.Cos(phi0)

	// The pin positions come from the exact coordinates reported by the
	// geolocation API; the country table is only a fallback. The home pin
	// is the location resolved at startup (red); while a tunnel is up the
	// exit pin (blue) tracks the verified exit, and both stay visible so
	// the origin and the destination are distinguishable on the same map.
	var homeLat, homeLon float64
	home := m.geo
	if home.loaded && home.err == nil {
		if home.lat != 0 || home.lon != 0 {
			homeLat, homeLon = home.lat, home.lon
		} else if c, ok := countryCoords[strings.ToUpper(home.code)]; ok {
			homeLat, homeLon = c[1], c[0]
		}
	}
	var exitLat, exitLon float64
	live := m.liveGeoInfo()
	if live.loaded && live.err == nil && live.vpn {
		if live.lat != 0 || live.lon != 0 {
			exitLat, exitLon = live.lat, live.lon
		} else if c, ok := countryCoords[strings.ToUpper(live.code)]; ok {
			exitLat, exitLon = c[1], c[0]
		}
	}

	homeCol, homeRow, hasHome := 0, 0, false
	if homeLat != 0 || homeLon != 0 {
		if u, v, vis := projectPoint(homeLat, homeLon, lam0, sinP0, cosP0); vis {
			homeRow = int(math.Round(v * float64(r)))
			homeCol = int(math.Round(u * float64(r)))
			hasHome = true
		}
	}
	exitCol, exitRow, hasExit := 0, 0, false
	if exitLat != 0 || exitLon != 0 {
		if u, v, vis := projectPoint(exitLat, exitLon, lam0, sinP0, cosP0); vis {
			exitRow = int(math.Round(v * float64(r)))
			exitCol = int(math.Round(u * float64(r)))
			hasExit = true
		}
	}

	title := ""
	sub := ""
	if !home.loaded && !live.loaded {
		sub = "locating…"
	} else if home.code == "" && live.code == "" {
		sub = "..."
	} else {
		if hasHome && home.code != "" {
			title = styleMarker.Render("YOU") + " " + countryFlag(home.code)
		}
		if hasExit && live.code != "" {
			if title != "" {
				title += " "
			}
			title += styleMarkerVpn.Render("VPN") + " " + countryFlag(live.code)
		}
		if live.vpn && live.code != "" {
			sub = live.code
			if live.city != "" {
				sub += " · " + live.city
			} else if live.name != "" {
				sub += " · " + live.name
			}
			sub += " · via VPN"
		} else if home.code != "" {
			sub = home.code
			if home.city != "" {
				sub += " · " + home.city
			} else if home.name != "" {
				sub += " · " + home.name
			}
		}
	}
	if title == "" {
		title = styleGeo.Render("YOU")
	}

	lines := make([]string, 0, 2*r+3)
	lines = append(lines, " "+title)

	rr := float64(r)
	for j := -r; j <= r; j++ {
		var b strings.Builder
		b.WriteByte(' ')
		for i := -r; i <= r; i++ {
			u := float64(i) / rr
			v := float64(j) / rr
			d2 := u*u + v*v
			if d2 > 1 {
				b.WriteByte(' ')
				continue
			}
			if hasExit && i == exitCol && j == exitRow {
				b.WriteString(styleMarkerVpn.Render("●"))
				continue
			}
			if hasHome && i == homeCol && j == homeRow {
				b.WriteString(styleMarker.Render("●"))
				continue
			}
			w := math.Sqrt(1 - d2)
			// Invert the rotation: the point on the sphere seen at (u,v,w).
			x := u*sinP0 + w*cosP0
			y := v
			z := -u*cosP0 + w*sinP0
			lat := math.Asin(z) * 180 / math.Pi
			lon := math.Atan2(y, x)*180/math.Pi + lon0

			diffuse := u*sunDir[0] + v*sunDir[1] + w*sunDir[2]
			level := shadeLevel(diffuse)
			if landAt(lat, lon) {
				b.WriteString(globeRamps.land[level].Render("▓"))
			} else {
				b.WriteString(globeRamps.ocean[level].Render("░"))
			}
		}
		lines = append(lines, b.String())
	}
	lines = append(lines, " "+styleDim.Render(truncate(sub, width-2)))

	pad := lipgloss.NewStyle()
	for i, l := range lines {
		lines[i] = pad.Width(width).Render(truncate(l, width))
	}

	// Vertically centre the globe in the list body column so it does not hug
	// the title when the terminal is much taller than the globe. The panel
	// view aligns its own chrome, so centering is skipped there.
	if !m.connPanel {
		if bodyH := m.bodyHeight(); bodyH > len(lines) {
			padTop := (bodyH - len(lines)) / 2
			for i := 0; i < padTop; i++ {
				lines = append([]string{pad.Width(width).Render("")}, lines...)
			}
			for len(lines) < bodyH {
				lines = append(lines, pad.Width(width).Render(""))
			}
		}
	}

	m.globeCache = &globeView{lines: lines, width: width}
	return m.globeCache
}

// bodyHeight is the number of lines the list body occupies at the current
// frame layout (header + rows + spacer + detail pane).
func (m *model) bodyHeight() int {
	fl := m.frameLayout()
	n := fl.rows
	if fl.header {
		n++
	}
	if fl.blank {
		n++
	}
	if fl.detail {
		n += 3
	}
	return n
}

// projectPoint maps (lat, lon) degrees into view-space coordinates (u, v) and
// reports whether the point is on the visible hemisphere. The view is the
// inverse of the per-pixel rotation in globeView.
func projectPoint(lat, lon, lam0, sinP0, cosP0 float64) (u, v float64, visible bool) {
	phi := lat * math.Pi / 180
	lam := lon*math.Pi/180 - lam0
	cosPhi := math.Cos(phi)
	px := cosPhi * math.Cos(lam)
	py := cosPhi * math.Sin(lam)
	pz := math.Sin(phi)
	u = px*sinP0 - pz*cosP0
	v = py
	if u*u+v*v > 1 {
		return 0, 0, false
	}
	return u, v, true
}
