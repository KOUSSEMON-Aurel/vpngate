package cmd

import (
	"os"
	"strconv"
	"strings"
	"time"

	tw "github.com/olekukonko/tablewriter"

	"github.com/davegallant/vpngate/pkg/vpn"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&flagProxy, "proxy", "p", "", "provide a http/https proxy server to make requests through (i.e. http://127.0.0.1:8080)")
	listCmd.Flags().StringVarP(&flagSocks5Proxy, "socks5", "s", "", "provide a socks5 proxy server to make requests through (i.e. 127.0.0.1:1080)")
	listCmd.Flags().StringVar(&flagCountry, "country", "", "filter by country name or country code (i.e. Japan or jp)")
	listCmd.Flags().IntVar(&flagMaxPing, "max-ping", 0, "filter out servers with ping higher than this value")
	listCmd.Flags().IntVar(&flagMinScore, "min-score", 0, "filter out servers with score lower than this value")
	listCmd.Flags().StringVar(&flagProto, "proto", "", "filter by tunnel transport (tcp or udp)")
	listCmd.Flags().StringVar(&flagSource, "source", "", "filter by server source (vpngate or vpnbook)")
	listCmd.Flags().StringVar(&flagSort, "sort", "none", "sort by one of none, score, ping, country, hostname")
	listCmd.Flags().StringVarP(&flagOutput, "output", "o", outputTable, "output format: table, json, csv")
	listCmd.Flags().BoolVar(&flagRefresh, "refresh", false, "refresh the vpn server list cache before listing")
	listCmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "do not read from or write to the vpn server list cache")
	listCmd.Flags().BoolVar(&flagHealthCheck, "health-check", false, "probe servers with a real OpenVPN connection before listing")
	listCmd.Flags().IntVar(&flagHealthConcurrency, "health-concurrency", 10, "number of parallel health probes")
	listCmd.Flags().DurationVar(&flagHealthTimeout, "health-timeout", 5*time.Second, "per-server health probe timeout")
	listCmd.Flags().BoolVar(&flagTUI, "tui", true, "use the interactive TUI browser when on a terminal")
	listCmd.Flags().BoolVar(&flagWatch, "watch", true, "keep re-verifying server health in the background")
	listCmd.Flags().DurationVar(&flagWatchInterval, "watch-interval", 30*time.Second, "how often to re-verify servers in the background")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available vpn servers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateSortFlag(); err != nil {
			return err
		}
		if err := validateOutputFlag(); err != nil {
			return err
		}
		if err := validateProtoFlag(); err != nil {
			return err
		}

		vpnServers, err := vpn.GetListWithOptions(flagProxy, flagSocks5Proxy, vpn.ListOptions{Refresh: flagRefresh, NoCache: flagNoCache})
		if err != nil {
			return err
		}

		vpnServers = filterServers(vpnServers)
		sortServers(vpnServers)

		if flagTUI && terminalInteractive() && strings.EqualFold(flagOutput, outputTable) {
			return runTuiBrowse(cmd.Context(), *vpnServers, flagHealthConcurrency, flagHealthTimeout, flagWatchInterval)
		}

		probeResults := make(map[string]vpn.ProbeResult)
		if flagHealthCheck {
			probeResults = runProbe(cmd.Context(), *vpnServers, flagHealthConcurrency, flagHealthTimeout)
		}

		switch strings.ToLower(flagOutput) {
		case outputJSON:
			return writeServersJSON(vpnServers)
		case outputCSV:
			return writeServersCSV(vpnServers)
		}

		table := tw.NewWriter(os.Stdout)
		if flagHealthCheck {
			table.Header([]string{"#", "HostName", "Country", "Proto", "Source", "Ping", "Status", "Latency", "Score"})
		} else {
			table.Header([]string{"#", "HostName", "Country", "Proto", "Source", "Ping", "Score", "Speed", "Sessions", "Operator"})
		}

		for i, v := range *vpnServers {
			if flagHealthCheck {
				r := probeResults[v.HostName]
				err := table.Append([]string{strconv.Itoa(i + 1), v.HostName, v.CountryLong, v.Proto(), sourceLabel(v), v.Ping, probeStatusLabel(r.Status), probeLatencyLabel(r.LatencyMs), strconv.Itoa(v.Score)})
				if err != nil {
					return err
				}
			} else {
				if err := table.Append([]string{strconv.Itoa(i + 1), v.HostName, v.CountryLong, v.Proto(), sourceLabel(v), v.Ping, strconv.Itoa(v.Score), v.SpeedLabel(), strconv.Itoa(v.NumVpnSessions), truncateOperator(v.Operator)}); err != nil {
					return err
				}
			}
		}
		return table.Render()
	},
}
