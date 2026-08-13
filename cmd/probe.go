package cmd

import (
	"context"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/davegallant/vpngate/internal/tui"
	"github.com/davegallant/vpngate/pkg/vpn"
)

// terminalInteractive reports whether stdin/stdout are attached to a real
// terminal (as opposed to a pipe, file, or scripted run).
func terminalInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

// runProbe verifies every server with a real OpenVPN handshake probe.
func runProbe(ctx context.Context, servers []vpn.Server, concurrency int, timeout time.Duration) map[string]vpn.ProbeResult {
	log.Info().Msgf("Probing %d servers (timeout: %s, concurrency: %d)...", len(servers), timeout, concurrency)
	return vpn.ProbeServers(ctx, servers, concurrency, timeout)
}

// workingServers returns the servers verified as usable, ordered by real
// latency ascending.
func workingServers(servers []vpn.Server, results map[string]vpn.ProbeResult) []vpn.Server {
	working := make([]vpn.Server, 0, len(servers))
	for _, s := range servers {
		if r, ok := results[s.HostName]; ok && r.Status == vpn.ProbeWorking {
			working = append(working, s)
		}
	}

	sort.SliceStable(working, func(i, j int) bool {
		return results[working[i].HostName].LatencyMs < results[working[j].HostName].LatencyMs
	})
	return working
}

// probeStatusLabel renders a probe status for table output.
func probeStatusLabel(status vpn.ProbeStatus) string {
	if status == "" {
		return "-"
	}
	return string(status)
}

// probeLatencyLabel renders a latency for table output.
func probeLatencyLabel(ms int) string {
	if ms <= 0 {
		return "-"
	}
	return strconv.Itoa(ms) + "ms"
}

// quietLogs silences logger output while an interactive TUI owns the
// terminal, so concurrent probe progress lines cannot corrupt the rendered
// screen. It returns a function that restores the previous global level.
func quietLogs() func() {
	previous := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	return func() { zerolog.SetGlobalLevel(previous) }
}

// runTuiPicker opens the interactive server picker. The returned bool is
// false when the user quit without selecting a server.
func runTuiPicker(ctx context.Context, servers []vpn.Server) (vpn.Server, bool, error) {
	restore := quietLogs()
	defer restore()
	return tui.Run(ctx, tui.Options{
		Servers:     servers,
		Concurrency: flagHealthConcurrency,
		Timeout:     flagHealthTimeout,
		Interval:    flagWatchInterval,
		Mode:        tui.ModeSelect,
		Watch:       flagWatch,
	})
}

// runTuiBrowse opens the read-only live server browser.
func runTuiBrowse(ctx context.Context, servers []vpn.Server, concurrency int, timeout, interval time.Duration) error {
	restore := quietLogs()
	defer restore()
	_, _, err := tui.Run(ctx, tui.Options{
		Servers:     servers,
		Concurrency: concurrency,
		Timeout:     timeout,
		Interval:    interval,
		Mode:        tui.ModeBrowse,
		Watch:       flagWatch,
	})
	return err
}
