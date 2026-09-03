package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/davegallant/vpngate/pkg/vpn"
)

// displayServers returns the servers for the current view: working servers
// first sorted by real latency, then the rest, optionally filtered to only
// verified-working ones and/or by the active search term.
func (m *model) displayServers() []vpn.Server {
	servers := make([]vpn.Server, len(m.servers))
	copy(servers, m.servers)

	// Freeze the sort order for the duration of a probe round, but re-derive
	// when a full round completes or when newly verified working servers
	// are found during round 0 so working relays immediately surface to the top.
	workingCount := 0
	for _, r := range m.results {
		if r.Status == vpn.ProbeWorking {
			workingCount++
		}
	}
	needReorder := m.orderRound != m.round || len(m.order) != len(servers) || (m.round == 0 && workingCount != m.orderWorking)

	if needReorder {
		sort.SliceStable(servers, func(i, j int) bool {
			return m.better(servers[i], servers[j])
		})
		m.order = m.order[:0]
		for _, s := range servers {
			m.order = append(m.order, s.HostName)
		}
		m.orderRound = m.round
		m.orderWorking = workingCount
	} else {
		byHost := make(map[string]int, len(servers))
		for i := range servers {
			byHost[servers[i].HostName] = i
		}
		ordered := make([]vpn.Server, 0, len(servers))
		for _, hn := range m.order {
			if i, ok := byHost[hn]; ok {
				ordered = append(ordered, servers[i])
			}
		}
		servers = ordered
	}

	if m.workingOnly {
		out := make([]vpn.Server, 0, len(servers))
		for _, s := range servers {
			if r, ok := m.results[s.HostName]; ok && r.Status == vpn.ProbeWorking {
				out = append(out, s)
			}
		}
		servers = out
	}

	if m.filter != "" {
		f := strings.ToLower(m.filter)
		out := make([]vpn.Server, 0, len(servers))
		for _, s := range servers {
			needle := strings.ToLower(s.HostName + " " + s.IPAddr + " " + s.CountryLong + " " + s.CountryShort + " " + s.Source)
			if strings.Contains(needle, f) {
				out = append(out, s)
			}
		}
		servers = out
	}

	return servers
}

// better orders two servers under the active sort mode. The default order
// keeps working relays first (by real latency, then hostname). The best and
// worst modes rank every server by the latency/score match instead, so a
// fast relay with a high score surfaces at the top regardless of where it
// sits in the CSV.
func (m *model) better(a, b vpn.Server) bool {
	ra, rb := m.results[a.HostName], m.results[b.HostName]
	switch m.sortMode {
	case sortModeBest, sortModeWorst:
		ma, mb := matchMetric(a, ra), matchMetric(b, rb)
		if ma != mb {
			if m.sortMode == sortModeBest {
				return ma < mb
			}
			return ma > mb
		}
		return a.HostName < b.HostName
	default:
		ka, kb := statusInfoFor(ra.Status).rank, statusInfoFor(rb.Status).rank
		if ka != kb {
			return ka < kb
		}
		if ka == 0 {
			la, lb := vpn.LatencyRank(ra.LatencyMs), vpn.LatencyRank(rb.LatencyMs)
			if la != lb {
				return la < lb
			}
			if a.Score != b.Score {
				return a.Score > b.Score
			}
			return a.HostName < b.HostName
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.HostName < b.HostName
	}
}

// matchMetric is the latency/score match value: lower is better, so a fast
// relay with a high score ranks first. Servers without a usable measurement
// (still being probed, or a zero score) sort behind every measured one.
func matchMetric(s vpn.Server, r vpn.ProbeResult) float64 {
	if r.Status != vpn.ProbeWorking || r.LatencyMs <= 0 || s.Score <= 0 {
		return math.Inf(1)
	}
	return float64(r.LatencyMs) / float64(s.Score)
}

// statusCounts tallies the current probe results.
func (m *model) statusCounts() (done, working, checking, down int) {
	for _, r := range m.results {
		switch r.Status {
		case vpn.ProbeWorking:
			done++
			working++
		case vpn.ProbeChecking, "":
			checking++
		case vpn.ProbeUnreachable, vpn.ProbeTimeout, vpn.ProbeError:
			done++
			down++
		case vpn.ProbeAuthFailed:
			done++
		}
	}
	return done, working, checking, down
}

func progressBar(done, total int) string {
	if total <= 0 {
		return ""
	}
	const width = 12
	pct := float64(done) / float64(total)
	if pct > 1 {
		pct = 1
	}
	fill := int(pct*width + 0.5)
	if fill > width {
		fill = width
	}
	bar := strings.Repeat("█", fill) + strings.Repeat("░", width-fill)
	style := styleDown
	switch {
	case pct >= 1:
		style = styleWorking
	case pct >= 0.3:
		style = styleWarn
	}
	return style.Render(bar) + fmt.Sprintf(" %2.0f%%", pct*100)
}
