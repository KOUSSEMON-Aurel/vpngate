package tui

import (
	"fmt"
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

	sort.SliceStable(servers, func(i, j int) bool {
		ri, rj := m.results[servers[i].HostName], m.results[servers[j].HostName]
		ki, kj := statusInfoFor(ri.Status).rank, statusInfoFor(rj.Status).rank
		if ki != kj {
			return ki < kj
		}
		if ki == 0 {
			return ri.LatencyMs < rj.LatencyMs
		}
		return servers[i].HostName < servers[j].HostName
	})

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
			needle := strings.ToLower(s.HostName + " " + s.IPAddr + " " + s.CountryLong + " " + s.CountryShort)
			if strings.Contains(needle, f) {
				out = append(out, s)
			}
		}
		servers = out
	}

	return servers
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
