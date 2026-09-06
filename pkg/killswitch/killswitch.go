package killswitch

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"github.com/davegallant/vpngate/pkg/vpn"
)

// ErrUnsupportedPlatform is returned when the current OS lacks a native
// packet filtering implementation.
var ErrUnsupportedPlatform = errors.New("kill switch is not supported on this operating system")

// KillSwitch defines the cross-platform contract for preventing network
// traffic leaks outside the VPN tunnel.
type KillSwitch interface {
	// Enable sets up pre-connection isolation rules allowing only the
	// VPN gateway handshake, loopback, and local network routes.
	Enable(ctx context.Context, s vpn.Server) error

	// OnTunnelUp restricts all outgoing system traffic exclusively to the
	// newly established tunnel interface (e.g., tun0, utun0) and loopback.
	OnTunnelUp(ctx context.Context, dev string) error

	// Disable tears down all packet filter rules and restores default
	// network connectivity.
	Disable(ctx context.Context) error

	// IsActive reports whether the kill switch protection is currently engaged.
	IsActive() bool

	// Name returns the underlying mechanism identifier (e.g. "linux-netns-nftables", "macos-pf", "windows-netsh").
	Name() string
}

// Endpoint represents a resolved VPN remote destination.
type Endpoint struct {
	IP    string
	Port  int
	Proto string
}

// ExtractEndpoint parses the destination IP, port, and transport protocol
// from a Server's embedded configuration or metadata fields.
func ExtractEndpoint(s vpn.Server) Endpoint {
	ep := Endpoint{
		IP:    s.IPAddr,
		Port:  1194,
		Proto: "udp",
	}

	if p := s.Proto(); p != "" {
		ep.Proto = strings.ToLower(p)
		if ep.Proto == "tcp" {
			ep.Port = 443
		}
	}

	if s.Transport != "" {
		t := strings.ToLower(s.Transport)
		if strings.HasPrefix(t, "tcp") {
			ep.Proto = "tcp"
			if portNum, err := strconv.Atoi(strings.TrimPrefix(t, "tcp")); err == nil && portNum > 0 {
				ep.Port = portNum
			}
		} else if strings.HasPrefix(t, "udp") {
			ep.Proto = "udp"
			if portNum, err := strconv.Atoi(strings.TrimPrefix(t, "udp")); err == nil && portNum > 0 {
				ep.Port = portNum
			}
		}
	}

	// If OpenVPN configuration data is available, parse the 'remote' directive
	if s.OpenVpnConfigData != "" {
		if decoded, err := base64.StdEncoding.DecodeString(s.OpenVpnConfigData); err == nil {
			for _, line := range strings.Split(string(decoded), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 3 && fields[0] == "remote" {
					if port, err := strconv.Atoi(fields[2]); err == nil && port > 0 {
						ep.Port = port
					}
					if len(fields) >= 4 {
						ep.Proto = strings.ToLower(fields[3])
					}
					break
				}
			}
		}
	}

	return ep
}
