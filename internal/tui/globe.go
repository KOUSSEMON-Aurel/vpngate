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

// globeRamps are the shaded colour ramps used to fake a single light source.
// Index 0 is the darkest (terminator), index 3 the brightest (sun side).
var globeRamps = struct {
	land  [4]lipgloss.Style
	ocean [4]lipgloss.Style
}{
	land: [4]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("34")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("70")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("76")),
	},
	ocean: [4]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("17")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("24")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("31")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
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

// globeView builds the orthographically projected, sun-shaded globe with the
// user's location pinned. It returns nil when the terminal is too narrow or
// short to fit it.
func (m *model) globeView() *globeView {
	rows := m.visibleRows()
	if m.width < 80 || rows < 7 {
		return nil
	}
	// The globe is sized to the room left after the column header, the
	// blank line and - when shown - the 3-line detail pane.
	r := (rows - 1) / 2
	if m.height >= 16 {
		r = (rows + 2) / 2
	}
	if r > 7 {
		r = 7
	}
	width := 2*r + 3

	lon0 := float64(m.globeRot % 360)
	phi0, lam0 := 15*math.Pi/180, lon0*math.Pi/180
	sinP0, cosP0 := math.Sin(phi0), math.Cos(phi0)

	markerCol, markerRow := 0, 0
	hasMarker := false
	if m.geo.loaded && m.geo.err == nil && m.geo.code != "" {
		if c, ok := countryCoords[strings.ToUpper(m.geo.code)]; ok {
			if u, v, vis := projectPoint(c[1], c[0], lam0, sinP0, cosP0); vis {
				markerRow = int(math.Round(v * float64(r)))
				markerCol = int(math.Round(u * float64(r)))
				hasMarker = true
			}
		}
	}

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
			if hasMarker && i == markerCol && j == markerRow {
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
	return &globeView{lines: lines, width: width}
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
