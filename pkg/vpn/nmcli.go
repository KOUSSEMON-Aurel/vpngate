package vpn

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/juju/errors"
	"github.com/rs/zerolog/log"
)

var nmImportPattern = regexp.MustCompile(`Connection '([^']+)' \(([0-9a-fA-F\-]{36})\) successfully added`)

// IsNetworkManagerAvailable reports whether NetworkManager (via nmcli) is installed,
// running, and responsive on Linux.
func IsNetworkManagerAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := exec.LookPath("nmcli"); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nmcli", "general", "status")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "running")
}

// ParseNMImportOutput parses the connection name and UUID from nmcli import output.
func ParseNMImportOutput(output string) (name, uuid string, err error) {
	matches := nmImportPattern.FindStringSubmatch(output)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract UUID from nmcli output: %q", strings.TrimSpace(output))
	}
	return matches[1], matches[2], nil
}

// ImportTemporaryNMConfig imports an OpenVPN config file as an in-memory/temporary
// NetworkManager profile. It returns the profile name and UUID.
func ImportTemporaryNMConfig(ctx context.Context, configPath string) (name, uuid string, err error) {
	cmd := exec.CommandContext(ctx, "nmcli", "connection", "import", "--temporary", "type", "openvpn", "file", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", errors.Annotatef(err, "nmcli connection import failed: %s", strings.TrimSpace(string(out)))
	}
	name, uuid, err = ParseNMImportOutput(string(out))
	if err != nil {
		return "", "", errors.Annotate(err, "parsing nmcli import output")
	}
	log.Debug().Msgf("nmcli: imported temporary connection %s (%s)", name, uuid)
	return name, uuid, nil
}

// ActivateNM activates a NetworkManager connection by UUID or name.
func ActivateNM(ctx context.Context, uuidOrName string) error {
	cmd := exec.CommandContext(ctx, "nmcli", "connection", "up", uuidOrName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Annotatef(err, "activating NM connection %s: %s", uuidOrName, strings.TrimSpace(string(out)))
	}
	log.Info().Msgf("nmcli: connection %s activated successfully", uuidOrName)
	return nil
}

// DeactivateNM deactivates an active NetworkManager connection.
func DeactivateNM(ctx context.Context, uuidOrName string) error {
	cmd := exec.CommandContext(ctx, "nmcli", "connection", "down", uuidOrName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Annotatef(err, "deactivating NM connection %s: %s", uuidOrName, strings.TrimSpace(string(out)))
	}
	log.Debug().Msgf("nmcli: connection %s deactivated", uuidOrName)
	return nil
}

// DeleteNM deletes a NetworkManager connection profile.
func DeleteNM(ctx context.Context, uuidOrName string) error {
	cmd := exec.CommandContext(ctx, "nmcli", "connection", "delete", uuidOrName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Annotatef(err, "deleting NM connection %s: %s", uuidOrName, strings.TrimSpace(string(out)))
	}
	log.Debug().Msgf("nmcli: connection %s deleted", uuidOrName)
	return nil
}

// NMClient connects to an OpenVPN relay using NetworkManager (nmcli) on Linux.
type NMClient struct{}

// Connect imports the server config into NetworkManager as a temporary connection,
// activates it, and waits until ctx is canceled before deactivating and deleting it.
func (NMClient) Connect(ctx context.Context, server Server, out io.Writer) error {
	if !IsNetworkManagerAvailable() {
		return errors.New("NetworkManager (nmcli) is not available or not running on this system")
	}
	configPath, err := WriteServerConfig(server)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(configPath) }()

	name, uuid, err := ImportTemporaryNMConfig(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = DeactivateNM(context.Background(), uuid)
		_ = DeleteNM(context.Background(), uuid)
	}()

	if err := ActivateNM(ctx, uuid); err != nil {
		return err
	}
	if out != nil {
		_, _ = fmt.Fprintf(out, "NetworkManager: connected to %s (%s)\n", name, uuid)
	}

	<-ctx.Done()
	return nil
}

// ConnectDetached falls back to openVPNClient for detached background execution
// with a management socket, while still allowing callers to manage lifecycle.
func (NMClient) ConnectDetached(server Server, configPath, managementAddr string, logWriter io.Writer, sysProcAttr *syscall.SysProcAttr) (*exec.Cmd, error) {
	return openVPNClient{}.ConnectDetached(server, configPath, managementAddr, logWriter, sysProcAttr)
}

