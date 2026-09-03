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
	c, _ := ClientForProtocol(server, "")
	return c
}

// ClientForProtocol returns the Client used to connect to server with the
// given VPN protocol. An empty protocol picks the server's primary one
// (OpenVPN for servers whose protocol cannot be determined). It errors
// when the protocol is unknown or not supported by the server's source
// (e.g. vpnbook has no L2TP/IPsec relays; WARP is wireguard only).
func ClientForProtocol(server Server, protocol string) (Client, error) {
	if protocol == "" {
		protocol = server.Protocol()
	}
	if protocol == "" {
		protocol = ProtocolOpenVPN
	}

	switch server.Source {
	case SourceWarp:
		if protocol == ProtocolWireGuard {
			return warpClient{}, nil
		}
		return nil, fmt.Errorf("WARP only supports the wireguard protocol, not %q", protocol)
	case SourceVpnbook:
		if protocol != ProtocolOpenVPN {
			return nil, fmt.Errorf("%s relays only support the openvpn protocol, not %q", server.Source, protocol)
		}
		return openVPNClient{}, nil
	}

	switch protocol {
	case ProtocolOpenVPN:
		return openVPNClient{}, nil
	case ProtocolL2TPIPsec:
		return l2tpClient{}, nil
	case ProtocolSSTP:
		return sstpClient{}, nil
	}
	return nil, fmt.Errorf("unsupported protocol %q", protocol)
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
	args := []string{"--verb", strconv.Itoa(verb), "--config", configPath}
	if !configDeclaresDataCiphers(server) {
		// Legacy relay configs (vpngate) only carry a "cipher" directive,
		// which OpenVPN 2.6 does not negotiate, so the classic cipher is
		// forced for them. Providers whose configs declare data-ciphers
		// (vpnbook) negotiate their own list instead: overriding it here
		// would break the handshake when the relay does not offer the
		// forced cipher.
		args = append(args, "--data-ciphers", "AES-128-CBC")
	}
	credsFile, err := authUserPassFileFor(server.Source)
	if err != nil {
		return nil, err
	}
	if credsFile != "" {
		args = append(args, "--auth-user-pass", credsFile)
	}
	if HostRoutesIPv6() {
		args = append(args, "--block-ipv6")
	}
	return args, nil
}

// configDeclaresDataCiphers reports whether the server's embedded OpenVPN
// config carries an active data-ciphers directive, i.e. the provider
// already controls cipher negotiation and the client must not override it.
func configDeclaresDataCiphers(server Server) bool {
	config, err := base64.StdEncoding.DecodeString(server.OpenVpnConfigData)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(config), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "data-ciphers" {
			return true
		}
	}
	return false
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

// ServerConfig decodes the server's embedded OpenVPN config and appends
// directives the client needs on top of it:
//
//   - a TCP MSS cap, because community relays (vpngate, vpnbook) ship
//     configs without mssfix and silently drop the oversized packets their
//     tunnels produce, so TCP dies while ICMP still passes;
//   - an IPv6 black hole when the host has a default IPv6 route. vpngate
//     relays are IPv4-only: on such a host browsers would otherwise bypass
//     the tunnel over IPv6 and reveal the real location; on IPv6-less hosts
//     (where no leak is possible) the directive is skipped so openvpn does
//     not warn about an IPv6 route it cannot apply.
func ServerConfig(server Server) ([]byte, error) {
	if server.OpenVpnConfigData == "" {
		return nil, errors.New("server has no embedded OpenVPN config")
	}
	decoded, err := base64.StdEncoding.DecodeString(server.OpenVpnConfigData)
	if err != nil {
		return nil, err
	}
	out := append(decoded, []byte("\n# vpngate: cap the TCP MSS so oversized segments fit through the community relays (their tunnels drop packets that exceed the path MTU)\nmssfix 1350\n")...)
	if !HostRoutesIPv6() {
		return out, nil
	}
	return append(out, []byte("# vpngate: block IPv6 leaks on dual-stack hosts (relays are IPv4-only)\nifconfig-ipv6 fd15:53b6:dead::2/64 fd15:53b6:dead::1\nredirect-gateway ipv6\nblock-ipv6\nroute-ipv6 ::/0\n")...), nil
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
