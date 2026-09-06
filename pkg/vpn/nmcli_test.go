package vpn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNMImportOutput(t *testing.T) {
	output := "Connection 'vpngate_japan_123' (d2251f9b-aea6-45f0-845e-d9080b0bdc7a) successfully added.\n"
	name, uuid, err := ParseNMImportOutput(output)
	assert.NoError(t, err)
	assert.Equal(t, "vpngate_japan_123", name)
	assert.Equal(t, "d2251f9b-aea6-45f0-845e-d9080b0bdc7a", uuid)

	invalid := "Error: failed to import"
	_, _, err = ParseNMImportOutput(invalid)
	assert.Error(t, err)
}

func TestIsNetworkManagerAvailable(t *testing.T) {
	// Should run without panic on any platform
	avail := IsNetworkManagerAvailable()
	t.Logf("NetworkManager available: %v", avail)
}
