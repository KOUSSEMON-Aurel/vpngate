package vpn

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/juju/errors"
)

const (
	// freevpnConfigProtocol is the single OpenVPN profile we keep for the
	// freevpn.me server (the bundle ships one config per transport).
	freevpnConfigProtocol = "tcp443"

	// freevpnCredsCacheFile holds the last seen credentials together with
	// a timestamp so they can be reused across connects without
	// re-scraping.
	freevpnCredsCacheFile = "freevpn_credentials.json"

	// freevpnCredsFile is the two-line auth-user-pass file referenced by
	// the OpenVPN config of the freevpn.me server.
	freevpnCredsFile = "freevpn.creds"

	// freevpnCredsCacheTTL is how long cached credentials are trusted
	// before re-scraping freevpn.me (the password rotates occasionally).
	freevpnCredsCacheTTL = 48 * time.Hour

	freevpnUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
)

// freevpnBaseURL and freevpnBundleURL are vars so tests can point them at
// a local httptest.Server instead of the real endpoints.
var (
	freevpnBaseURL   = "https://freevpn.me/accounts/"
	freevpnBundleURL = "https://freevpn.me/FreeVPN.me-OpenVPN-Bundle-July-2020.zip"
)

// FreevpnCredentials are the shared credentials used by the freevpn.me
// server.
type FreevpnCredentials struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	freevpnValueRe = regexp.MustCompile(`(?:\*\*|</[^>]+>|\s)+([A-Za-z0-9._-]+)`)
	freevpnZipRe   = regexp.MustCompile(`https://freevpn\.me/[^"'<>\)\s]+\.zip`)
)

// FetchFreevpnServers scrapes the freevpn.me accounts page for the shared
// credentials and the OpenVPN bundle link, then downloads the bundle and
// extracts the OpenVPN config for freevpnConfigProtocol.
func FetchFreevpnServers(client *http.Client) ([]Server, error) {
	payload, err := fetchFreevpnPage(client)
	if err != nil {
		return nil, err
	}

	creds := extractFreevpnCredentials(payload)
	if creds.Username != "" && creds.Password != "" {
		if cacheErr := cacheFreevpnCredentials(creds); cacheErr != nil {
			log.Warn().Msgf("freevpn: unable to cache credentials: %s", cacheErr)
		}
	}

	bundleURL := freevpnBundleURL
	if match := freevpnZipRe.Find(payload); match != nil {
		bundleURL = string(match)
	}

	config, err := fetchFreevpnConfig(client, bundleURL)
	if err != nil {
		return nil, err
	}

	return []Server{{
		HostName:          "server1.freevpn.me",
		CountryLong:       "Netherlands",
		CountryShort:      "nl",
		IPAddr:            "server1.freevpn.me",
		OpenVpnConfigData: base64.StdEncoding.EncodeToString(config),
		Source:            SourceFreevpn,
	}}, nil
}

// FetchFreevpnCredentials scrapes the current freevpn.me credentials from
// the accounts page.
func FetchFreevpnCredentials(client *http.Client) (FreevpnCredentials, error) {
	payload, err := fetchFreevpnPage(client)
	if err != nil {
		return FreevpnCredentials{}, err
	}

	creds := extractFreevpnCredentials(payload)
	if creds.Username == "" || creds.Password == "" {
		return FreevpnCredentials{}, errors.New("freevpn: unable to extract credentials from page")
	}
	return creds, nil
}

var freevpnCredsMu sync.Mutex

// GetFreevpnCredentials returns current freevpn.me credentials, preferring
// the cached copy when it is fresh to avoid hammering freevpn.me on every
// connect. Concurrent callers are serialized so only one scrape happens.
func GetFreevpnCredentials(client *http.Client) (FreevpnCredentials, error) {
	freevpnCredsMu.Lock()
	defer freevpnCredsMu.Unlock()

	if creds, ok := readFreevpnCredsCache(); ok {
		return creds, nil
	}

	log.Info().Msg("freevpn: fetching credentials from freevpn.me")
	creds, err := FetchFreevpnCredentials(client)
	if err != nil {
		return FreevpnCredentials{}, err
	}
	if err := cacheFreevpnCredentials(creds); err != nil {
		log.Warn().Msgf("freevpn: unable to cache credentials: %s", err)
	}
	return creds, nil
}

// WriteFreevpnCredsFile writes the credentials in OpenVPN's two-line
// auth-user-pass format and returns the path of the written file.
func WriteFreevpnCredsFile(creds FreevpnCredentials) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(cacheDir, freevpnCredsFile)
	if err := os.WriteFile(path, []byte(creds.Username+"\n"+creds.Password+"\n"), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// freevpnCredsFileFor returns the path of a two-line auth-user-pass file
// holding the current freevpn.me credentials, reusing the on-disk cache
// and only scraping freevpn.me when the cache is stale.
func freevpnCredsFileFor() (string, error) {
	creds, err := GetFreevpnCredentials(defaultHTTPClient())
	if err != nil {
		return "", err
	}
	return WriteFreevpnCredsFile(creds)
}

func fetchFreevpnPage(client *http.Client) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, freevpnBaseURL, nil)
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to create request")
	}
	req.Header.Set("User-Agent", freevpnUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to fetch page")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("freevpn: unexpected status code: %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to read page")
	}
	return payload, nil
}

// fetchFreevpnConfig downloads the OpenVPN bundle and returns the raw
// config for freevpnConfigProtocol (falling back to the first .ovpn in
// the bundle when that transport is missing).
func fetchFreevpnConfig(client *http.Client, bundleURL string) ([]byte, error) {
	resp, err := client.Get(bundleURL)
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to download bundle")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("freevpn: unexpected bundle status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to read bundle")
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to open bundle")
	}

	var fallback *zip.File
	want := strings.ToLower(freevpnConfigProtocol)
	for _, file := range zipReader.File {
		if !strings.HasSuffix(strings.ToLower(file.Name), ".ovpn") {
			continue
		}
		if fallback == nil {
			fallback = file
		}
		if strings.Contains(strings.ToLower(file.Name), want) {
			return readZipFile(file)
		}
	}
	if fallback == nil {
		return nil, errors.New("freevpn: no .ovpn config found in bundle")
	}
	return readZipFile(fallback)
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, errors.Annotate(err, "freevpn: unable to open config in bundle")
	}
	defer func() {
		_ = rc.Close()
	}()
	return io.ReadAll(rc)
}

// extractFreevpnCredentials reads the username and password from the
// accounts page. In the page each credential is rendered as the label
// (e.g. "Username:") followed by the value; take the first value after
// the label.
func extractFreevpnCredentials(payload []byte) FreevpnCredentials {
	creds := FreevpnCredentials{
		UpdatedAt: time.Now(),
	}
	creds.Username = freevpnLabelValue(payload, "Username")
	creds.Password = freevpnLabelValue(payload, "Password")
	return creds
}

func freevpnLabelValue(payload []byte, label string) string {
	idx := bytes.Index(payload, []byte(label+":"))
	if idx < 0 {
		return ""
	}
	window := payload[idx+len(label)+1:]
	if len(window) > 200 {
		window = window[:200]
	}
	match := freevpnValueRe.FindSubmatch(window)
	if match == nil {
		return ""
	}
	return string(match[1])
}

func cacheFreevpnCredentials(creds FreevpnCredentials) error {
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
	return os.WriteFile(filepath.Join(cacheDir, freevpnCredsCacheFile), data, 0o600)
}

func readFreevpnCredsCache() (FreevpnCredentials, bool) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return FreevpnCredentials{}, false
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, freevpnCredsCacheFile))
	if err != nil {
		return FreevpnCredentials{}, false
	}
	var creds FreevpnCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return FreevpnCredentials{}, false
	}
	if creds.Username == "" || creds.Password == "" {
		return FreevpnCredentials{}, false
	}
	if time.Since(creds.UpdatedAt) > freevpnCredsCacheTTL {
		return FreevpnCredentials{}, false
	}
	return creds, true
}
