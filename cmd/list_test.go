package cmd

import (
	"encoding/base64"
	"testing"

	"github.com/davegallant/vpngate/pkg/vpn"
	"github.com/stretchr/testify/assert"
)

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func resetFilterFlags() {
	flagCountry = ""
	flagMaxPing = 0
	flagMinScore = 0
	flagProto = ""
	flagSource = ""
}

func TestFilterServersByCountryCode(t *testing.T) {
	defer resetFilterFlags()
	servers := []vpn.Server{
		{HostName: "a", CountryShort: "jp", CountryLong: "Japan"},
		{HostName: "b", CountryShort: "us", CountryLong: "United States"},
	}

	flagCountry = "jp"
	got := filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "a", (*got)[0].HostName)
}

func TestFilterServersByCountryNameSubstring(t *testing.T) {
	defer resetFilterFlags()
	servers := []vpn.Server{
		{HostName: "a", CountryShort: "jp", CountryLong: "Japan"},
	}

	flagCountry = "Japan"
	got := filterServers(&servers)
	assert.Len(t, *got, 1)
}

func TestFilterServersByMinScore(t *testing.T) {
	defer resetFilterFlags()
	servers := []vpn.Server{
		{HostName: "a", Score: 50},
		{HostName: "b", Score: 150},
	}

	flagMinScore = 100
	got := filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "b", (*got)[0].HostName)
}

func TestFilterServersByMaxPing(t *testing.T) {
	defer resetFilterFlags()
	servers := []vpn.Server{
		{HostName: "a", Ping: "50"},
		{HostName: "b", Ping: "200"},
		{HostName: "c", Ping: "not-a-number"},
	}

	flagMaxPing = 100
	got := filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "a", (*got)[0].HostName)
}

func TestPingSortValueInvalid(t *testing.T) {
	assert.Equal(t, int(^uint(0)>>1), pingSortValue("not-a-number"))
}

func TestPingSortValueValid(t *testing.T) {
	assert.Equal(t, 42, pingSortValue("42"))
}

func TestSortServersByScore(t *testing.T) {
	defer func() { flagSort = "" }()
	servers := []vpn.Server{
		{HostName: "low", Score: 10},
		{HostName: "high", Score: 100},
	}

	flagSort = "score"
	sortServers(&servers)
	assert.Equal(t, "high", servers[0].HostName)
}

func TestSortServersByPing(t *testing.T) {
	defer func() { flagSort = "" }()
	servers := []vpn.Server{
		{HostName: "slow", Ping: "500"},
		{HostName: "fast", Ping: "10"},
	}

	flagSort = "ping"
	sortServers(&servers)
	assert.Equal(t, "fast", servers[0].HostName)
}

func TestValidateSortFlag(t *testing.T) {
	defer func() { flagSort = "" }()

	flagSort = "score"
	assert.NoError(t, validateSortFlag())

	flagSort = "bogus"
	assert.Error(t, validateSortFlag())
}

func TestValidateOutputFlag(t *testing.T) {
	defer func() { flagOutput = "" }()

	flagOutput = outputTable
	assert.NoError(t, validateOutputFlag())

	flagOutput = "bogus"
	assert.Error(t, validateOutputFlag())
}

func TestValidateProtoFlag(t *testing.T) {
	defer resetFilterFlags()

	assert.NoError(t, validateProtoFlag())

	flagProto = "TCP"
	assert.NoError(t, validateProtoFlag())

	flagProto = "bogus"
	assert.Error(t, validateProtoFlag())
}

func TestFilterServersByProto(t *testing.T) {
	defer resetFilterFlags()
	servers := []vpn.Server{
		{HostName: "tcp-relay", OpenVpnConfigData: base64Encode("client\nproto tcp\nremote 1.2.3.4 1458")},
		{HostName: "udp-relay", OpenVpnConfigData: base64Encode("client\nproto udp\nremote 1.2.3.4 1194")},
		{HostName: "unknown-relay", OpenVpnConfigData: ""},
	}

	flagProto = "tcp"
	got := filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "tcp-relay", (*got)[0].HostName)

	flagProto = "udp"
	got = filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "udp-relay", (*got)[0].HostName)
}

func TestFilterServersBySource(t *testing.T) {
	defer resetFilterFlags()
	servers := []vpn.Server{
		{HostName: "a", Source: vpn.SourceVpngate},
		{HostName: "b", Source: vpn.SourceVpnbook},
		{HostName: "c"},
	}

	flagSource = "vpnbook"
	got := filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "b", (*got)[0].HostName)

	flagSource = "VPNGATE"
	got = filterServers(&servers)
	assert.Len(t, *got, 1)
	assert.Equal(t, "a", (*got)[0].HostName)
}
