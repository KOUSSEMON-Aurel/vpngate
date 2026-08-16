package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/davegallant/vpngate/pkg/vpn"
)

func init() {
	warpCmd.Flags().StringVar(&flagWgcfConfig, "wgcf-config", "", "path to an existing wgcf WireGuard profile (default: <config>/wgcf/wgcf-profile.conf)")
	rootCmd.AddCommand(warpCmd)
}

// flagWgcfConfig overrides the wgcf WireGuard profile path used by the
// wgcf backend.
var flagWgcfConfig string

var warpCmd = &cobra.Command{
	Use:   "warp",
	Short: "Connect to Cloudflare WARP (wgcf + wg-quick, or warp-cli)",
	Long: "Connect to Cloudflare WARP. Uses wgcf to register an account, generate a\n" +
		"WireGuard profile and bring it up with wg-quick when both are installed;\n" +
		"otherwise falls back to the official warp-cli daemon. The wgcf path needs\n" +
		"root (it creates a tun interface).",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWarp()
	},
}

// runWarp connects to Cloudflare WARP in the foreground, verifying the
// tunnel end-to-end once it is up. It blocks until interrupted.
func runWarp() error {
	backend, err := vpn.DetectWarpBackend()
	if err != nil {
		return err
	}
	fmt.Printf("WARP backend: %s\n", backend)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return vpn.WarpConnect(ctx, flagWgcfConfig, os.Stdout, func() {
		verifyTunnel(func(s string) { fmt.Println(s) })
	})
}
