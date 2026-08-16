package vpn

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jszwec/csvutil"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"

	"github.com/davegallant/vpngate/pkg/util"
	"github.com/juju/errors"
)

const (
	// httpClientTimeout must be generous: the vpngate relay list is a
	// multi-megabyte download that frequently crawls at well under
	// 100 KB/s, and a tight timeout made fetching fail in practice.
	httpClientTimeout = 120 * time.Second
	dialTimeout       = 10 * time.Second
	fetchRetryDelay   = time.Second
	fetchRetryCount   = 3

	// SourceVpngate identifies relays advertised by vpngate.net.
	SourceVpngate = "vpngate"
	// SourceVpnbook identifies servers advertised by vpnbook.com.
	SourceVpnbook = "vpnbook"
	// SourceWarp identifies the Cloudflare WARP tunnel.
	SourceWarp = "warp"

	// ProtocolOpenVPN connects through the openvpn binary with the
	// server's embedded config.
	ProtocolOpenVPN = "openvpn"
	// ProtocolL2TPIPsec connects through strongswan (IPsec) + xl2tpd
	// (L2TP) with the shared VPN Gate L2TP credentials.
	ProtocolL2TPIPsec = "l2tp/ipsec"
	// ProtocolSSTP connects through sstp-client with the shared VPN Gate
	// SSTP credentials.
	ProtocolSSTP = "sstp"
	// ProtocolWireGuard connects through wg-quick (WARP only).
	ProtocolWireGuard = "wireguard"
)

// vpnList is the URL of the vpngate server list API. It is a var so tests
// can point it at a local httptest.Server instead of the real endpoint.
var vpnList = "https://www.vpngate.net/api/iphone/"

// Server holds information about a vpn relay server
type Server struct {
	HostName          string `csv:"#HostName"`
	CountryLong       string `csv:"CountryLong"`
	CountryShort      string `csv:"CountryShort"`
	Score             int    `csv:"Score"`
	IPAddr            string `csv:"IP"`
	OpenVpnConfigData string `csv:"OpenVPN_ConfigData_Base64"`
	Ping              string `csv:"Ping"`
	LatencyMs         int    `csv:"-" json:"latency_ms,omitempty"`
	// Speed is the advertised relay bandwidth in bytes per second.
	Speed int64 `csv:"Speed"`
	// NumVpnSessions is the number of active VPN sessions on the relay.
	NumVpnSessions int `csv:"NumVpnSessions"`
	// Uptime is the relay's uptime in seconds.
	Uptime int64 `csv:"Uptime"`
	// TotalUsers is the cumulative number of users served by the relay.
	TotalUsers int64 `csv:"TotalUsers"`
	// TotalTraffic is the cumulative bytes carried by the relay.
	TotalTraffic int64 `csv:"TotalTraffic"`
	// LogType is how the relay operator logs sessions (e.g. "2weeks").
	LogType string `csv:"LogType"`
	// Operator is the volunteer running the relay, e.g. "Daiyuu Nobori_
	// Japan. Academic Use Only.".
	Operator string `csv:"Operator"`
	// Message is an optional operator-supplied note about the relay.
	Message string `csv:"Message"`
	// Source is the provider that advertised this server, e.g. SourceVpngate
	// or SourceVpnbook. It is not part of the vpngate CSV payload.
	Source string `csv:"-" json:"source,omitempty"`
	// Transport is the OpenVPN transport the server connects over by default
	// (e.g. "tcp443" for vpnbook servers). It is empty for sources whose
	// transport is fixed by the embedded config (vpngate relays are always
	// reached over their config's tcp/udp proto).
	Transport string `csv:"-" json:"transport,omitempty"`
}

// Proto returns the tunnel transport ("tcp" or "udp") declared in the
// relay's OpenVPN configuration, or "" when it cannot be determined.
// vpngate relays listen on either UDP (the default) or TCP, and an ISP may
// handle the two transports very differently, so callers can filter on it.
func (s Server) Proto() string {
	if s.OpenVpnConfigData == "" {
		return ""
	}

	config, err := base64.StdEncoding.DecodeString(s.OpenVpnConfigData)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(config), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "proto" {
			switch fields[1] {
			case "tcp", "tcp-client", "tcp4", "tcp6":
				return "tcp"
			case "udp", "udp4", "udp6":
				return "udp"
			}
		}
	}

	return ""
}

// Protocol returns the primary VPN protocol a server connects with:
// "openvpn" for relays carrying an embedded OpenVPN config (vpngate,
// vpnbook) and "wireguard" for Cloudflare WARP. It returns "" when the
// protocol cannot be determined. Distinct from Proto, which is the
// OpenVPN tunnel transport (tcp or udp). Use Protocols to see every
// protocol a server supports.
func (s Server) Protocol() string {
	if ps := s.Protocols(); len(ps) > 0 {
		return ps[0]
	}
	return ""
}

// Protocols returns every VPN protocol a server can be connected with,
// in decreasing order of reliability. VPN Gate software exposes OpenVPN,
// L2TP/IPsec and MS-SSTP on every relay, so all three are offered for
// vpngate relays even though the public CSV only carries the OpenVPN
// config — the L2TP and SSTP credentials are shared and fixed for the
// whole pool, so neither needs per-server data.
func (s Server) Protocols() []string {
	switch s.Source {
	case SourceWarp:
		return []string{ProtocolWireGuard}
	case SourceVpnbook:
		return []string{ProtocolOpenVPN}
	case SourceVpngate:
		return []string{ProtocolOpenVPN, ProtocolL2TPIPsec, ProtocolSSTP}
	}
	if s.OpenVpnConfigData != "" {
		return []string{ProtocolOpenVPN}
	}
	return nil
}

// ProtocolLabel renders the server's supported protocols for list
// output (e.g. "openvpn, l2tp/ipsec, sstp").
func (s Server) ProtocolLabel() string {
	return strings.Join(s.Protocols(), ", ")
}

// TransportLabel returns the OpenVPN transport the server connects over
// by default: the embedded default for vpnbook servers ("tcp443"), or
// the tcp/udp proto declared in the embedded OpenVPN config for other
// sources. It returns "" when the transport cannot be determined.
func (s Server) TransportLabel() string {
	switch s.Source {
	case SourceVpnbook:
		if s.Transport != "" {
			return s.Transport
		}
		return TransportTCP443
	}
	return s.Proto()
}

// Transports returns the OpenVPN transports a server can be connected
// over, or nil when the transport is fixed by the embedded config.
// vpnbook serves every server over all four transports; vpngate relays
// and WARP are always reached the way their config declares.
func (s Server) Transports() []string {
	switch s.Source {
	case SourceVpnbook:
		return VpnbookTransports()
	}
	return nil
}

// SpeedLabel returns the relay's advertised speed as a short human-readable
// string (e.g. "279.8 MB/s"), or "" when the value is missing.
func (s Server) SpeedLabel() string {
	if s.Speed <= 0 {
		return ""
	}
	return humanBytes(s.Speed)
}

// UptimeLabel returns the relay's uptime as a short human-readable string
// (e.g. "2d 4h"), or "" when the value is missing.
func (s Server) UptimeLabel() string {
	if s.Uptime <= 0 {
		return ""
	}
	days := s.Uptime / (24 * 3600)
	hours := (s.Uptime % (24 * 3600)) / 3600
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dh", hours)
}

// TrafficLabel returns the relay's cumulative traffic as a short
// human-readable string (e.g. "593.7 TB"), or "" when the value is missing.
func (s Server) TrafficLabel() string {
	if s.TotalTraffic <= 0 {
		return ""
	}
	return humanBytes(s.TotalTraffic)
}

// humanBytes renders a byte count using binary-ish units (KB, MB, GB, TB).
func humanBytes(n int64) string {
	switch {
	case n >= 1<<40:
		return fmt.Sprintf("%.1f TB", float64(n)/(1<<40))
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// parseVpnList parses the VPN server list from CSV format
func parseVpnList(r io.Reader) (*[]Server, error) {
	var servers []Server

	serverList, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Annotate(err, "Unable to read stream")
	}

	// Trim known invalid rows
	serverList = bytes.TrimPrefix(serverList, []byte("*vpn_servers\r\n"))
	serverList = bytes.TrimSuffix(serverList, []byte("*\r\n"))
	serverList = bytes.ReplaceAll(serverList, []byte(`"`), []byte{})

	if err := csvutil.Unmarshal(serverList, &servers); err != nil {
		return nil, errors.Annotatef(err, "Unable to parse CSV")
	}

	for i := range servers {
		if alias, ok := countryAliases[servers[i].CountryLong]; ok {
			servers[i].CountryLong = alias
		}
		servers[i].Source = SourceVpngate
	}

	return &servers, nil
}

// countryAliases maps vpngate.net's CountryLong values to more familiar
// country names.
var countryAliases = map[string]string{
	"Korea Republic of":  "South Korea",
	"Russian Federation": "Russia",
}

// createHTTPClient creates an HTTP client with optional proxy configuration
func createHTTPClient(httpProxy string, socks5Proxy string) (*http.Client, error) {
	if httpProxy != "" {
		proxyURL, err := url.Parse(httpProxy)
		if err != nil {
			return nil, errors.Annotatef(err, "Error parsing HTTP proxy: %s", httpProxy)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		return &http.Client{
			Transport: transport,
			Timeout:   httpClientTimeout,
		}, nil
	}

	if socks5Proxy != "" {
		dialer, err := proxy.SOCKS5("tcp", socks5Proxy, nil, proxy.Direct)
		if err != nil {
			return nil, errors.Annotatef(err, "Error creating SOCKS5 dialer: %v", err)
		}

		// Create a DialContext function from the SOCKS5 dialer
		dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Check if context is already done
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			// Use the dialer with a timeout
			conn, err := dialer.Dial(network, addr)
			if err != nil {
				return nil, err
			}

			// Respect context cancellation after connection
			go func() {
				<-ctx.Done()
				_ = conn.Close()
			}()

			return conn, nil
		}

		httpTransport := &http.Transport{
			DialContext: dialContext,
		}
		return &http.Client{
			Transport: httpTransport,
			Timeout:   httpClientTimeout,
		}, nil
	}

	return &http.Client{
		Timeout: httpClientTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: dialTimeout,
			}).DialContext,
		},
	}, nil
}

// ListOptions controls how the vpn server list is fetched.
type ListOptions struct {
	Refresh bool
	NoCache bool

	// DisableVpnbook opts out of merging vpnbook.com OpenVPN servers into
	// the returned list. Merging is enabled by default.
	DisableVpnbook bool
}

// mergeVpnbookServers appends vpnbook.com servers to the vpngate list. It
// returns nil on failure so the caller can keep working with vpngate only.
func mergeVpnbookServers(client *http.Client, vpngateServers *[]Server) *[]Server {
	vpnbookServers, err := FetchVpnbookServers(client)
	if err != nil {
		log.Warn().Msgf("Unable to fetch vpnbook servers: %s", err)
		return nil
	}

	merged := make([]Server, 0, len(*vpngateServers)+len(vpnbookServers))
	merged = append(merged, (*vpngateServers)...)
	merged = append(merged, vpnbookServers...)
	return &merged
}

// GetList returns a list of vpn servers.
func GetList(httpProxy string, socks5Proxy string) (*[]Server, error) {
	return GetListWithOptions(httpProxy, socks5Proxy, ListOptions{})
}

// GetListWithOptions returns a list of vpn servers with cache controls.
func GetListWithOptions(httpProxy string, socks5Proxy string, opts ListOptions) (*[]Server, error) {
	cacheExpired := vpnListCacheIsExpired()

	// Try to use cached list if not expired, unless explicitly bypassed.
	if !opts.Refresh && !opts.NoCache && !cacheExpired {
		servers, err := getVpnListCache()
		if err == nil {
			return servers, nil
		}
		log.Info().Msg("Unable to retrieve vpn list from cache")
	} else if opts.Refresh {
		log.Info().Msg("Refreshing the vpn server list")
	} else if opts.NoCache {
		log.Info().Msg("Bypassing the vpn server list cache")
	} else {
		log.Info().Msg("The vpn server list cache has expired")
	}

	log.Info().Msg("Fetching the latest server list (the download can take a while)")

	client, err := createHTTPClient(httpProxy, socks5Proxy)
	if err != nil {
		return nil, err
	}

	var servers *[]Server

	err = util.Retry(fetchRetryCount, fetchRetryDelay, func() error {
		resp, err := client.Get(vpnList)
		if err != nil {
			return err
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("Unexpected status code when retrieving vpn list: %d", resp.StatusCode)
		}

		parsedServers, err := parseVpnList(resp.Body)
		if err != nil {
			return err
		}

		servers = parsedServers

		// Merge vpnbook.com servers into the list, unless opted out.
		if !opts.DisableVpnbook {
			if merged := mergeVpnbookServers(client, servers); merged != nil {
				servers = merged
			}
		}

		// Cloudflare WARP is always available: it needs no relay list, so
		// it is appended last and stays reachable even when every relay
		// fetch fails.
		*servers = append(*servers, WarpServer())

		// Cache the servers for future use, unless caching is disabled.
		if !opts.NoCache {
			cacheErr := writeVpnListToCache(*servers)
			if cacheErr != nil {
				log.Warn().Msgf("Unable to write servers to cache: %s", cacheErr)
			}
		}
		return nil
	})

	if err != nil {
		if opts.NoCache {
			return nil, err
		}
		// Fall back to whatever is cached, even if stale, rather than
		// failing outright when the remote list cannot be fetched.
		if servers, cacheErr := getVpnListCache(); cacheErr == nil {
			log.Warn().Msgf("Using cached vpn server list after fetch failure: %s", err)
			return servers, nil
		}
		return nil, err
	}

	return servers, nil
}
