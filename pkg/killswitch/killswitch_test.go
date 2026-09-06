package killswitch

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/davegallant/vpngate/pkg/vpn"
)

func TestExtractEndpointDefaults(t *testing.T) {
	server := vpn.Server{
		IPAddr: "198.51.100.1",
	}

	ep := ExtractEndpoint(server)
	assert.Equal(t, "198.51.100.1", ep.IP)
	assert.Equal(t, 1194, ep.Port)
	assert.Equal(t, "udp", ep.Proto)
}

func TestExtractEndpointTransport(t *testing.T) {
	server := vpn.Server{
		IPAddr:    "198.51.100.2",
		Transport: "tcp443",
	}

	ep := ExtractEndpoint(server)
	assert.Equal(t, "198.51.100.2", ep.IP)
	assert.Equal(t, 443, ep.Port)
	assert.Equal(t, "tcp", ep.Proto)
}

func TestExtractEndpointConfigOverride(t *testing.T) {
	config := "client\ndev tun\nproto udp\nremote 203.0.113.5 1195\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(config))

	server := vpn.Server{
		IPAddr:            "198.51.100.3",
		OpenVpnConfigData: encoded,
	}

	ep := ExtractEndpoint(server)
	assert.Equal(t, "198.51.100.3", ep.IP)
	assert.Equal(t, 1195, ep.Port)
	assert.Equal(t, "udp", ep.Proto)
}

func TestNewKillSwitch(t *testing.T) {
	ks := New()
	assert.NotNil(t, ks)
	assert.NotEmpty(t, ks.Name())
	assert.False(t, ks.IsActive())
}
