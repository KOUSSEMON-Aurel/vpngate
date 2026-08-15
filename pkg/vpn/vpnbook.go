package vpn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/juju/errors"
)

const (
	// vpnbookConfigProtocol is the single OpenVPN profile we keep per server
	// so each server appears once in the merged list.
	vpnbookConfigProtocol = "tcp443"

	// vpnbookFetchConcurrency bounds the number of in-flight config fetches.
	vpnbookFetchConcurrency = 4

	vpnbookConfigFetchRetries = 2

	// vpnbookCredsCacheFile holds the last seen credentials together with a
	// timestamp so they can be reused across connects without re-scraping.
	vpnbookCredsCacheFile = "vpnbook_credentials.json"

	// vpnbookCredsFile is the two-line auth-user-pass file referenced by the
	// OpenVPN configs of vpnbook servers.
	vpnbookCredsFile = "vpnbook.creds"

	// vpnbookCredsCacheTTL is how long cached credentials are trusted before
	// re-scraping vpnbook.com (the password rotates every few weeks).
	vpnbookCredsCacheTTL = 48 * time.Hour

	vpnbookUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
)

// vpnbookBaseURL and vpnbookConfigURL are vars so tests can point them at a
// local httptest.Server instead of the real endpoints.
var (
	vpnbookBaseURL   = "https://www.vpnbook.com/freevpn/openvpn"
	vpnbookConfigURL = "https://www.vpnbook.com/api/openvpn"
)

// vpnbookFlightServer is a server entry inside the Next.js RSC payload served
// at vpnbookBaseURL.
type vpnbookFlightServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	IPAddress   string `json:"ipAddress"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
}

// VpnbookCredentials are the shared credentials used by every vpnbook server.
type VpnbookCredentials struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FetchVpnbookServers scrapes the vpnbook server list and downloads each
// server's OpenVPN config. The returned servers use the flight id as
// HostName (the canonical display key) and the FQDN only inside the config.
func FetchVpnbookServers(client *http.Client) ([]Server, error) {
	payload, err := fetchVpnbookPayload(client)
	if err != nil {
		return nil, err
	}

	flightServers, err := parseVpnbookServers(payload)
	if err != nil {
		return nil, err
	}

	if creds := extractVpnbookCredentials(payload); creds.Username != "" && creds.Password != "" {
		if cacheErr := cacheVpnbookCredentials(creds); cacheErr != nil {
			log.Warn().Msgf("vpnbook: unable to cache credentials: %s", cacheErr)
		}
	}

	servers := make([]Server, len(flightServers))

	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, vpnbookFetchConcurrency)
	)

	for index, flightServer := range flightServers {
		wg.Add(1)
		go func(index int, flightServer vpnbookFlightServer) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			config, err := fetchVpnbookConfig(client, flightServer.Hostname)
			if err != nil {
				log.Warn().Msgf("vpnbook: %s: %s", flightServer.ID, err)
				return
			}

			servers[index] = Server{
				HostName:          flightServer.ID,
				CountryLong:       flightServer.CountryName,
				CountryShort:      flightServer.CountryCode,
				IPAddr:            flightServer.IPAddress,
				OpenVpnConfigData: base64.StdEncoding.EncodeToString(config),
				Source:            SourceVpnbook,
			}
		}(index, flightServer)
	}
	wg.Wait()

	// Drop servers whose config fetch failed, keeping page order.
	filtered := servers[:0]
	for _, server := range servers {
		if server.HostName != "" {
			filtered = append(filtered, server)
		}
	}

	return filtered, nil
}

// FetchVpnbookCredentials scrapes the current vpnbook credentials from the
// vpnbook site.
func FetchVpnbookCredentials(client *http.Client) (VpnbookCredentials, error) {
	payload, err := fetchVpnbookPayload(client)
	if err != nil {
		return VpnbookCredentials{}, err
	}

	creds := extractVpnbookCredentials(payload)
	if creds.Username == "" || creds.Password == "" {
		return VpnbookCredentials{}, errors.New("vpnbook: unable to extract credentials from page")
	}
	return creds, nil
}

var vpnbookCredsMu sync.Mutex

// GetVpnbookCredentials returns current vpnbook credentials, preferring the
// cached copy when it is fresh to avoid hammering vpnbook.com on every
// connect. Concurrent callers are serialized so only one scrape happens.
func GetVpnbookCredentials(client *http.Client) (VpnbookCredentials, error) {
	vpnbookCredsMu.Lock()
	defer vpnbookCredsMu.Unlock()

	if creds, ok := readVpnbookCredsCache(); ok {
		return creds, nil
	}

	log.Info().Msg("vpnbook: fetching credentials from vpnbook.com")
	creds, err := FetchVpnbookCredentials(client)
	if err != nil {
		return VpnbookCredentials{}, err
	}
	if err := cacheVpnbookCredentials(creds); err != nil {
		log.Warn().Msgf("vpnbook: unable to cache credentials: %s", err)
	}
	return creds, nil
}

// WriteVpnbookCredsFile writes the credentials in OpenVPN's two-line
// auth-user-pass format and returns the path of the written file.
func WriteVpnbookCredsFile(creds VpnbookCredentials) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(cacheDir, vpnbookCredsFile)
	if err := os.WriteFile(path, []byte(creds.Username+"\n"+creds.Password+"\n"), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func fetchVpnbookPayload(client *http.Client) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, vpnbookBaseURL, nil)
	if err != nil {
		return nil, errors.Annotate(err, "vpnbook: unable to create request")
	}
	req.Header.Set("RSC", "1")
	req.Header.Set("User-Agent", vpnbookUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "vpnbook: unable to fetch page")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("vpnbook: unexpected status code: %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "vpnbook: unable to read page")
	}
	return payload, nil
}

func fetchVpnbookConfig(client *http.Client, hostname string) ([]byte, error) {
	params := url.Values{}
	params.Set("hostname", hostname)
	params.Set("protocol", vpnbookConfigProtocol)
	configURL := fmt.Sprintf("%s?%s", vpnbookConfigURL, params.Encode())

	var lastErr error
	for attempt := 0; attempt < vpnbookConfigFetchRetries; attempt++ {
		resp, err := client.Get(configURL)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = errors.Errorf("unexpected status code: %d", resp.StatusCode)
			continue
		}
		if len(bytes.TrimSpace(body)) == 0 {
			lastErr = errors.New("empty config")
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

func parseVpnbookServers(payload []byte) ([]vpnbookFlightServer, error) {
	serversJSON, err := extractVpnbookServersJSON(payload)
	if err != nil {
		return nil, err
	}

	var flightServers []vpnbookFlightServer
	if err := json.Unmarshal(serversJSON, &flightServers); err != nil {
		return nil, errors.Annotate(err, "vpnbook: unable to parse server list")
	}
	return flightServers, nil
}

// extractVpnbookServersJSON finds the `"servers":[...]` array in the RSC
// payload and returns its balanced JSON text.
func extractVpnbookServersJSON(payload []byte) ([]byte, error) {
	const marker = `"servers":`
	idx := bytes.Index(payload, []byte(marker))
	if idx < 0 {
		return nil, errors.New("vpnbook: server list not found in payload")
	}
	rest := payload[idx+len(marker):]
	if len(rest) == 0 || rest[0] != '[' {
		return nil, errors.New("vpnbook: malformed server list")
	}

	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return rest[:i+1], nil
			}
		}
	}
	return nil, errors.New("vpnbook: unbalanced server list")
}

var vpnbookChildrenRe = regexp.MustCompile(`children":"([^"]+)"`)

// extractVpnbookCredentials reads the username and password from the RSC
// payload. In the flight data each credential pair is rendered as the label
// (e.g. "Username") followed by the value; take the first child value that is
// not the label.
func extractVpnbookCredentials(payload []byte) VpnbookCredentials {
	creds := VpnbookCredentials{
		UpdatedAt: time.Now(),
	}
	creds.Username = vpnbookChildrenValue(payload, "Username")
	creds.Password = vpnbookChildrenValue(payload, "Password")
	return creds
}

func vpnbookChildrenValue(payload []byte, label string) string {
	idx := bytes.Index(payload, []byte(label))
	if idx < 0 {
		return ""
	}
	window := payload[idx:]
	if len(window) > 600 {
		window = window[:600]
	}
	for _, match := range vpnbookChildrenRe.FindAllSubmatch(window, -1) {
		value := string(match[1])
		if value != label {
			return value
		}
	}
	return ""
}

func cacheVpnbookCredentials(creds VpnbookCredentials) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, vpnbookCredsCacheFile), data, 0o600)
}

func readVpnbookCredsCache() (VpnbookCredentials, bool) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return VpnbookCredentials{}, false
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, vpnbookCredsCacheFile))
	if err != nil {
		return VpnbookCredentials{}, false
	}
	var creds VpnbookCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return VpnbookCredentials{}, false
	}
	if creds.Username == "" || creds.Password == "" {
		return VpnbookCredentials{}, false
	}
	if time.Since(creds.UpdatedAt) > vpnbookCredsCacheTTL {
		return VpnbookCredentials{}, false
	}
	return creds, true
}
