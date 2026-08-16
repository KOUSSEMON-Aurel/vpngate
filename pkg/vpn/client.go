package vpn

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/davegallant/vpngate/pkg/exec"
)

// executablePath returns the platform-specific path to the openvpn
// binary.
func executablePath() string {
	if runtime.GOOS == "windows" {
		return `C:\Program Files\OpenVPN\bin\openvpn.exe`
	}
	return "openvpn"
}

// Client connects to a VPN server using the tunnel implementation that
// matches the server's Source.
type Client interface {
	// Connect runs the tunnel for server until it exits or ctx is
	// canceled, streaming openvpn output to out (when non-nil).
	Connect(ctx context.Context, server Server, out io.Writer) error

	// ConnectDetached starts the tunnel for server using the config at
	// configPath with a management interface enabled at managementAddr,
	// detached via sysProcAttr so it outlives the calling process. Its
	// combined stdout/stderr are written to logWriter. It returns as soon
	// as the process has started; callers wait on the returned *exec.Cmd
	// independently (via cmd.Wait()) to learn when it exits.
	ConnectDetached(server Server, configPath, managementAddr string, logWriter io.Writer, sysProcAttr *syscall.SysProcAttr) (*osexec.Cmd, error)
}

// ClientFor returns the Client used to connect to server, selected by its
// Source. Unknown sources fall back to OpenVPN.
func ClientFor(server Server) Client {
	if server.Source == SourceWarp {
		return warpClient{}
	}
	return openVPNClient{}
}

// openVPNClient tunnels through the openvpn binary.
type openVPNClient struct{}

func (openVPNClient) Connect(ctx context.Context, server Server, out io.Writer) error {
	configPath, err := WriteServerConfig(server)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(configPath) }()

	verb := 4
	if os.Getenv("VPNGATE_DEBUG") != "" {
		verb = 5
		log.Debug().Msgf("debug: connecting with verbosity %d to %s (%s)", verb, server.HostName, server.IPAddr)
	}

	args, err := openvpnArgs(server, configPath, verb)
	if err != nil {
		return err
	}
	return exec.RunContext(ctx, executablePath(), ".", out, args...)
}

func (openVPNClient) ConnectDetached(server Server, configPath, managementAddr string, logWriter io.Writer, sysProcAttr *syscall.SysProcAttr) (*osexec.Cmd, error) {
	executable := executablePath()
	if _, err := osexec.LookPath(executable); err != nil {
		return nil, fmt.Errorf("%s is required, please install it", executable)
	}

	host, port, err := net.SplitHostPort(managementAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid management address %q: %w", managementAddr, err)
	}

	args, err := openvpnArgs(server, configPath, 4)
	if err != nil {
		return nil, err
	}
	args = append(args, "--management", host, port)

	cmd := osexec.Command(executable, args...)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	cmd.SysProcAttr = sysProcAttr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// openvpnArgs builds the openvpn argv for server. Configs from providers
// that use shared credentials carry a bare "auth-user-pass" directive that
// would otherwise block on stdin, so the credentials file is passed
// explicitly.
func openvpnArgs(server Server, configPath string, verb int) ([]string, error) {
	args := []string{"--verb", strconv.Itoa(verb), "--config", configPath, "--data-ciphers", "AES-128-CBC"}
	credsFile, err := authUserPassFileFor(server.Source)
	if err != nil {
		return nil, err
	}
	if credsFile != "" {
		args = append(args, "--auth-user-pass", credsFile)
	}
	return args, nil
}

// authUserPassFileFor returns the path of a two-line auth-user-pass file
// for the provider that requires shared credentials (vpnbook),
// or "" when the source does not use one.
func authUserPassFileFor(source string) (string, error) {
	switch source {
	case SourceVpnbook:
		return vpnbookCredsFileFor()
	}
	return "", nil
}

// vpnbookCredsFileFor returns the path of a two-line auth-user-pass file
// holding the current vpnbook credentials, reusing the on-disk cache and
// only scraping vpnbook.com when the cache is stale.
func vpnbookCredsFileFor() (string, error) {
	creds, err := GetVpnbookCredentials(defaultHTTPClient())
	if err != nil {
		return "", err
	}
	return WriteVpnbookCredsFile(creds)
}

// defaultHTTPClient is the proxy-less client used for credential lookups
// that happen outside the server-list fetch (where the proxy flags apply).
func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: httpClientTimeout}
}

// ServerConfig decodes the server's embedded OpenVPN config, appending a
// directive that folds IPv6 into the tunnel when the host has a default
// IPv6 route. vpngate relays are IPv4-only: on such a host browsers would
// otherwise bypass the tunnel over IPv6 and reveal the real location; on
// IPv6-less hosts (where no leak is possible) the directive is skipped so
// openvpn does not warn about an IPv6 route it cannot apply.
func ServerConfig(server Server) ([]byte, error) {
	if server.OpenVpnConfigData == "" {
		return nil, errors.New("server has no embedded OpenVPN config")
	}
	decoded, err := base64.StdEncoding.DecodeString(server.OpenVpnConfigData)
	if err != nil {
		return nil, err
	}
	if !HostRoutesIPv6() {
		return decoded, nil
	}
	return append(decoded, []byte("\n# vpngate: force all IPv6 into the tunnel (relays are IPv4-only)\nroute-ipv6 ::/0\n")...), nil
}

// WriteServerConfig writes ServerConfig to a temporary file and returns its
// path. Callers must remove the file when done.
func WriteServerConfig(server Server) (string, error) {
	data, err := ServerConfig(server)
	if err != nil {
		return "", err
	}

	tmpfile, err := os.CreateTemp("", "vpngate-openvpn-config-")
	if err != nil {
		return "", err
	}

	if _, err := tmpfile.Write(data); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return "", err
	}

	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpfile.Name())
		return "", err
	}
	return tmpfile.Name(), nil
}

// HostRoutesIPv6 reports whether the host has a default IPv6 route, i.e.
// real IPv6 connectivity that could bypass the IPv4-only tunnel. It reads
// /proc/net/ipv6_route (Linux) and treats absence as "no IPv6" everywhere.
func HostRoutesIPv6() bool {
	raw, err := os.ReadFile("/proc/net/ipv6_route")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "00000000000000000000000000000000" && f[1] == "00" {
			return true
		}
	}
	return false
}
