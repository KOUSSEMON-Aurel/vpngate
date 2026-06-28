package vpn

import (
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type HealthResult struct {
	HostName  string
	Reachable bool
	LatencyMs int
}

func ParseRemoteFromConfig(base64Data string) (host string, port string, ok bool) {
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", "", false
	}

	for _, line := range strings.Split(string(decoded), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "remote ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[1], parts[2], true
			}
			if len(parts) >= 2 {
				return parts[1], "443", true
			}
		}
	}

	return "", "", false
}

func CheckServer(server *Server, timeout time.Duration) HealthResult {
	host, port, ok := ParseRemoteFromConfig(server.OpenVpnConfigData)
	if !ok {
		host = server.IPAddr
		port = "443"
	}

	addr := net.JoinHostPort(host, port)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return HealthResult{HostName: server.HostName, Reachable: false}
	}
	conn.Close()

	latency := time.Since(start)
	return HealthResult{
		HostName:  server.HostName,
		Reachable: true,
		LatencyMs: int(latency.Milliseconds()),
	}
}

func CheckServers(servers []Server, timeout time.Duration, concurrency int) map[string]HealthResult {
	results := make(map[string]HealthResult, len(servers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, concurrency)

	for i := range servers {
		wg.Add(1)
		sem <- struct{}{}

		go func(s *Server) {
			defer wg.Done()
			defer func() { <-sem }()

			result := CheckServer(s, timeout)

			mu.Lock()
			results[s.HostName] = result
			mu.Unlock()

			if result.Reachable {
				log.Debug().Msgf("%s (%s) is reachable (latency: %dms)", s.HostName, s.IPAddr, result.LatencyMs)
			} else {
				log.Debug().Msgf("%s (%s) is unreachable", s.HostName, s.IPAddr)
			}
		}(&servers[i])
	}

	wg.Wait()
	return results
}
