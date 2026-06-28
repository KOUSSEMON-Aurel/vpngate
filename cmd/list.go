package cmd

import (
	"os"
	"strconv"
	"strings"

	tw "github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog/log"

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
	listCmd.Flags().StringVar(&flagSort, "sort", "none", "sort by one of none, score, ping, country, hostname")
	listCmd.Flags().StringVarP(&flagOutput, "output", "o", outputTable, "output format: table, json, csv")
	listCmd.Flags().BoolVar(&flagRefresh, "refresh", false, "refresh the vpn server list cache before listing")
	listCmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "do not read from or write to the vpn server list cache")
	listCmd.Flags().BoolVar(&flagHealthCheck, "health-check", false, "test server reachability before listing")
	listCmd.Flags().IntVar(&flagHealthConcurrency, "health-concurrency", 20, "number of parallel health checks")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available vpn servers",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {

		if err := validateSortFlag(); err != nil {
			log.Fatal().Msg(err.Error())
		}
		if err := validateOutputFlag(); err != nil {
			log.Fatal().Msg(err.Error())
		}

		vpnServers, err := vpn.GetListWithOptions(flagProxy, flagSocks5Proxy, vpn.ListOptions{Refresh: flagRefresh, NoCache: flagNoCache})
		if err != nil {
			log.Fatal().Msg(err.Error())
		}

		vpnServers = filterServers(vpnServers)
		sortServers(vpnServers)

		if flagHealthCheck {
			log.Info().Msgf("Checking reachability of %d servers...", len(*vpnServers))
			healthResults := vpn.CheckServers(*vpnServers, 3*1000*1000*1000, flagHealthConcurrency)

			reachable := make([]vpn.Server, 0, len(*vpnServers))
			for _, s := range *vpnServers {
				if result, ok := healthResults[s.HostName]; ok && result.Reachable {
					s.LatencyMs = result.LatencyMs
					reachable = append(reachable, s)
				}
			}

			filtered := len(*vpnServers) - len(reachable)
			vpnServers = &reachable
			log.Info().Msgf("%d servers are reachable (%d filtered out as unreachable)", len(*vpnServers), filtered)
		}

		switch strings.ToLower(flagOutput) {
		case outputJSON:
			if err := writeServersJSON(vpnServers); err != nil {
				log.Fatal().Msg(err.Error())
			}
			return
		case outputCSV:
			if err := writeServersCSV(vpnServers); err != nil {
				log.Fatal().Msg(err.Error())
			}
			return
		}

		table := tw.NewWriter(os.Stdout)
		if flagHealthCheck {
			table.Header([]string{"#", "HostName", "Country", "Ping", "Latency", "Score"})
		} else {
			table.Header([]string{"#", "HostName", "Country", "Ping", "Score"})
		}

		for i, v := range *vpnServers {
			if flagHealthCheck {
				err := table.Append([]string{strconv.Itoa(i + 1), v.HostName, v.CountryLong, v.Ping, strconv.Itoa(v.LatencyMs) + "ms", strconv.Itoa(v.Score)})
				if err != nil {
					log.Fatal().Msg(err.Error())
				}
			} else {
				err := table.Append([]string{strconv.Itoa(i + 1), v.HostName, v.CountryLong, v.Ping, strconv.Itoa(v.Score)})
				if err != nil {
					log.Fatal().Msg(err.Error())
				}
			}
		}
		err = table.Render()
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	},
}
