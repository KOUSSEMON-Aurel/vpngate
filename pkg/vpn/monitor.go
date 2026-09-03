package vpn

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MonitorOptions controls how a Monitor verifies servers.
type MonitorOptions struct {
	Concurrency int
	Timeout     time.Duration
	Interval    time.Duration
	// Continuous keeps re-verifying in the background. When false the
	// monitor runs a single verification round and then stops.
	Continuous bool
}

// Monitor continuously re-verifies a set of servers in the background so
// the current state is always available (a working relay that dies is
// flagged as soon as its next probe round completes).
type Monitor struct {
	mu       sync.RWMutex
	servers  []Server
	results  map[string]ProbeResult
	rounds   uint64
	running     bool
	inFlight    bool
	paused      bool
	roundCancel context.CancelFunc

	concurrency int
	timeout     time.Duration
	interval    time.Duration
	continuous  bool

	ctx     context.Context
	cancel  context.CancelFunc
	trigger chan struct{}
	resume  chan struct{}
}

// NewMonitor builds a monitor for the given servers. The default timeout
// is 5s, interval 30s and concurrency 10 when not provided.
func NewMonitor(servers []Server, opts MonitorOptions) *Monitor {
	if opts.Concurrency < 1 {
		opts.Concurrency = 10
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.Interval <= 0 {
		opts.Interval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		servers:     servers,
		results:     make(map[string]ProbeResult, len(servers)),
		concurrency: opts.Concurrency,
		timeout:     opts.Timeout,
		interval:    opts.Interval,
		continuous:  opts.Continuous,
		ctx:         ctx,
		cancel:      cancel,
		trigger:     make(chan struct{}, 1),
		resume:      make(chan struct{}, 1),
	}
}

// Start begins the background verification loop. The initial probe round
// starts immediately.
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.loop()
}

// Pause suspends automatic verification rounds until Resume is called.
// In-flight probes are cancelled immediately so they do not run through
// the live VPN tunnel and report false failures.
func (m *Monitor) Pause() {
	m.mu.Lock()
	m.paused = true
	if m.roundCancel != nil {
		m.roundCancel()
		m.roundCancel = nil
	}
	// Clear any lingering "checking" status so servers don't spin while paused.
	for k, v := range m.results {
		if v.Status == ProbeChecking {
			delete(m.results, k)
		}
	}
	m.mu.Unlock()
}

// Resume re-enables automatic verification rounds.
func (m *Monitor) Resume() {
	m.mu.Lock()
	if !m.paused {
		m.mu.Unlock()
		return
	}
	m.paused = false
	m.mu.Unlock()
	select {
	case m.resume <- struct{}{}:
	default:
	}
	m.ForceRound()
}

func (m *Monitor) isPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.paused
}

// Stop halts the background loop and cancels any in-flight probes.
func (m *Monitor) Stop() {
	m.cancel()
}

func (m *Monitor) loop() {
	m.ForceRound()
	if !m.continuous {
		return
	}
	for {
		select {
		case <-m.trigger:
		case <-time.After(m.interval):
		case <-m.ctx.Done():
			return
		}
		if m.isPaused() {
			// Wait for Resume (or shutdown) without scheduling rounds:
			// probing through a live tunnel would be meaningless.
			select {
			case <-m.resume:
			case <-m.ctx.Done():
				return
			}
			continue
		}
		m.ForceRound()
	}
}

// ForceRound schedules an immediate verification round (coalesced if one is
// already running). While paused it is a no-op.
func (m *Monitor) ForceRound() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.paused {
		return
	}
	if m.inFlight {
		select {
		case m.trigger <- struct{}{}:
		default:
		}
		return
	}
	m.inFlight = true
	go m.runRound()
}

func (m *Monitor) runRound() {
	m.mu.Lock()
	if m.paused {
		m.inFlight = false
		m.mu.Unlock()
		return
	}
	roundCtx, roundCancel := context.WithCancel(m.ctx)
	m.roundCancel = roundCancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.inFlight = false
		roundCancel()
		if m.roundCancel != nil {
			m.roundCancel = nil
		}
		m.mu.Unlock()
	}()

	m.markChecking()
	results := ProbeServers(roundCtx, m.servers, m.concurrency, m.timeout, func(name string, res ProbeResult) {
		// Publish each completed probe immediately so consumers (such as
		// the TUI) stream statuses live while the round is still running.
		m.mu.Lock()
		if !m.paused {
			m.results[name] = res
		}
		m.mu.Unlock()
	})

	m.mu.Lock()
	if !m.paused {
		for name, res := range results {
			m.results[name] = res
		}
		m.rounds++
	}
	m.mu.Unlock()
}

func (m *Monitor) markChecking() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		if _, ok := m.results[s.HostName]; !ok {
			m.results[s.HostName] = ProbeResult{Status: ProbeChecking}
		}
	}
}

// Results returns a snapshot of the latest probe results keyed by hostname.
func (m *Monitor) Results() map[string]ProbeResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]ProbeResult, len(m.results))
	for k, v := range m.results {
		out[k] = v
	}
	return out
}

// Round returns the number of completed verification rounds.
func (m *Monitor) Round() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rounds
}

// WorkingServers returns the servers whose latest probe verified them as
// usable, sorted by real latency ascending.
func (m *Monitor) WorkingServers() []Server {
	m.mu.RLock()
	servers := make([]Server, 0, len(m.servers))
	for _, s := range m.servers {
		if r, ok := m.results[s.HostName]; ok && r.Status == ProbeWorking {
			servers = append(servers, s)
		}
	}
	latencies := make(map[string]int, len(servers))
	for _, s := range servers {
		latencies[s.HostName] = m.results[s.HostName].LatencyMs
	}
	m.mu.RUnlock()

	sort.SliceStable(servers, func(i, j int) bool {
		return latencies[servers[i].HostName] < latencies[servers[j].HostName]
	})
	return servers
}
