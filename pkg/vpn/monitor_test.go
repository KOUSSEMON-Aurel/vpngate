package vpn

import (
	"testing"
	"time"
)

func TestMonitorWorkingServersSorted(t *testing.T) {
	cleanup := fakeOpenVPN(t, "PUSH_REPLY,route 0.0.0.0 0.0.0.0")
	defer cleanup()

	host, port, stop := localTCPListener(t)
	defer stop()

	servers := []Server{
		serverWithConfig(host, port),
		serverWithConfig(host, port),
	}
	for i := range servers {
		servers[i].HostName = "mon-srv-" + string(rune('a'+i))
	}

	mon := NewMonitor(servers, MonitorOptions{
		Concurrency: 2,
		Timeout:     5 * time.Second,
		Interval:    time.Hour, // one-shot: effectively continuous=false
		Continuous:  false,
	})
	mon.Start()
	defer mon.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for working servers")
		}
		if len(mon.WorkingServers()) == len(servers) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if round := mon.Round(); round < 1 {
		t.Fatalf("expected at least one round, got %d", round)
	}

	// All servers are identical speed, so every entry must be working and
	// the slice sorted by latency (non-decreasing).
	ws := mon.WorkingServers()
	if len(ws) != len(servers) {
		t.Fatalf("expected %d working servers, got %d", len(servers), len(ws))
	}
	for i := 1; i < len(ws); i++ {
		if ws[i-1].LatencyMs > ws[i].LatencyMs {
			t.Fatalf("working servers not sorted by latency: %v", ws)
		}
	}
}

func TestMonitorStopsAfterOneShotRound(t *testing.T) {
	cleanup := fakeOpenVPN(t, "PUSH_REPLY,route 0.0.0.0 0.0.0.0")
	defer cleanup()

	host, port, stop := localTCPListener(t)
	defer stop()

	server := serverWithConfig(host, port)
	server.HostName = "one-shot"

	mon := NewMonitor([]Server{server}, MonitorOptions{
		Concurrency: 1,
		Timeout:     5 * time.Second,
		Interval:    20 * time.Millisecond,
		Continuous:  false,
	})
	mon.Start()
	defer mon.Stop()

	// One-shot monitor runs a single round then stops; the round count must
	// not keep climbing after the first verification.
	first := mon.Round()
	time.Sleep(200 * time.Millisecond)
	after := mon.Round()
	if after < 1 {
		t.Fatalf("expected at least one round, got %d", after)
	}
	if after-first > 1 {
		t.Fatalf("one-shot monitor kept running rounds: %d -> %d", first, after)
	}
}

func TestMonitorForceRound(t *testing.T) {
	cleanup := fakeOpenVPN(t, "PUSH_REPLY,route 0.0.0.0 0.0.0.0")
	defer cleanup()

	host, port, stop := localTCPListener(t)
	defer stop()

	server := serverWithConfig(host, port)
	server.HostName = "force-round"

	mon := NewMonitor([]Server{server}, MonitorOptions{
		Concurrency: 1,
		Timeout:     5 * time.Second,
		Interval:    time.Hour,
		Continuous:  false,
	})
	mon.Start()
	defer mon.Stop()

	time.Sleep(200 * time.Millisecond)
	first := mon.Round()

	done := make(chan struct{})
	go func() {
		mon.ForceRound()
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for forced round")
		}
		if mon.Round() > first {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	<-done
}
