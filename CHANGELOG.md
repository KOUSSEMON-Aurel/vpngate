# Changelog

## Unreleased

- Add vpnbook multi-transport: `connect --transport` selects one of the four vpnbook transports (`tcp443`, `tcp80`, `udp53`, `udp25000`) and fetches the matching OpenVPN config at connect time, `list` gains a Transport column (also in CSV/JSON and the TUI detail pane), and the daemon forwards the flag. Servers keep their embedded `tcp443` profile by default.
- Add L2TP/IPsec and MS-SSTP protocols for vpngate relays: `connect --protocol l2tp/ipsec` (strongSwan + xl2tpd, `vpn`/`vpn`) and `connect --protocol sstp` (sstp-client, `sstp://ip:443`), with the corresponding relay servers kept filterable by protocol. Relay addresses come from the CSV, no HTML scraping.
- Fix OpenVPN connects failing with `AUTH_FAILED` on providers whose configs declare their own `data-ciphers` (vpnbook): the forced `--data-ciphers AES-128-CBC` override is now only applied to legacy configs without a `data-ciphers` directive.
- Fix dead TCP egress through community relays: generated configs now cap the TCP MSS (`mssfix 1350`) because the relays silently drop the oversized packets their tunnels produce (ICMP and TCP handshakes worked, data transfers stalled).
- Remove the freevpn.me provider: its single upstream server no longer accepts connections.
- Add Cloudflare WARP as a first-class source: `vpngate warp` connects through wgcf (automatic account registration and WireGuard profile) or the warp-cli fallback, and the `warp` source appears in `list`/`connect` with a "working" probe marker (no relay to verify) so it sorts last in latency-based orders while remaining connectable explicitly.
- Add a one-liner installer (`curl -fsSL https://raw.githubusercontent.com/KOUSSEMON-Aurel/vpngate/main/install.sh | bash`) that installs the binary from the latest GitHub release (falling back to `go install`) and the runtime dependencies (openvpn, wireguard-tools, wgcf) via the detected package manager.
- Keep connected tunnels alive on relays with partial or flaky egress: the live-tunnel watchdog now probes several HTTPS endpoints in parallel (`gstatic`, `google`, `1.1.1.1`, `8.8.8.8` — two in pure IP) and treats any HTTP response as alive, instead of relying on a single `gstatic.com/generate_204` probe that produced false negatives and endless reconnect chains.
- Make the watchdog less aggressive: a 30s grace window after connect and a threshold of 5 consecutive failures (~80s) before a relay is considered dead, so a transient outage no longer drops a working tunnel.
- Add a TUI `p` key to pause the watchdog while connected (footer shows `[p] health on/off`) and a `--tunnel-health-check=false` flag to disable it entirely, so a connected tunnel is never dropped by vpngate.
- Regenerate the demo GIF and document the watchdog and its pause toggle in the README.

## 0.8.0

- Verify servers with a real OpenVPN probe before offering them: candidates are probed by actually running `openvpn` and only servers that complete the handshake (`PUSH_REPLY`) are considered usable. This filters out the many full/maintenance servers on vpngate.net that pass a plain TCP/TLS check but reject authentication (`AUTH_FAILED`).
- Add continuous background re-verification (`--watch`, default on, `--watch-interval` 30s) so server health stays current while the list is on screen.
- Add an interactive bubbletea TUI: `connect` opens a picker that only selects verified-working servers, `list` opens a live browser with status, latency, and score columns. Both fall back to plain output when not on a terminal.
- Add `connect --best` to automatically select the fastest verified-working server.
- Add `--health-concurrency` (10) and `--health-timeout` (5s) to tune probing.
- Replace the old TCP-ping health check (`pkg/vpn/health.go`) with the OpenVPN probe (`pkg/vpn/probe.go`) and a monitor (`pkg/vpn/monitor.go`), and add unit tests for both.
- Fix `connect`/`list` exiting with `log.Fatal` when the server list could not be fetched: errors now propagate through `RunE`.

## 0.7.0

- Add `vpngate logs` to view the log for a background connection started with `connect -d`, with `-f`/`--follow` and `-n`/`--lines` options.
- Simplify the interactive server selection list to show a country flag emoji, country name, and IP address (e.g. `🇯🇵 Japan (219.100.37.4)`), dropping the hostname/ping/score columns and column-alignment padding.
- Alias vpngate.net's `Korea Republic of` and `Russian Federation` country names to `South Korea` and `Russia`.

## 0.6.0

- Add `connect -d`/`--daemon` to run a vpn connection in the background.
- Add `vpngate status` to check on a background connection started with `connect -d`.
- Add `vpngate disconnect` to tear down a background connection started with `connect -d`.
- Add winget packaging for Windows (manifest publishing is currently disabled pending fork/token setup).
- Fix `connect -d` silently timing out with no useful error when OpenVPN isn't installed: the background supervisor now logs its own startup failures to `daemon.log` (previously discarded, since the detached process has no console), with the same "is required, please install it" message the foreground `connect` command already gives.

## 0.5.0

- Fix a nil-pointer panic when the vpngate.net server list API returns a non-200 status code.
- Fix the retry backoff between failed server-list fetch attempts, which was effectively instantaneous (1ns) instead of 1 second.
- Fix `connect --reconnect` handling so a single connection attempt (without `--reconnect`) no longer loops forever after a clean disconnect.
- Fix a potential deadlock when reading OpenVPN's stdout/stderr output.
- Fix a leftover temporary OpenVPN config file when writing or closing it failed.
- Return errors from CLI commands instead of calling `log.Fatal` directly, for cleaner and more consistent error output.
- Update golang.org/x/net to v0.55.0 [security].
- Add test coverage for retry logic and CLI helper functions.

## 0.4.0

- Add server filtering by country, maximum ping, and minimum score to list and connect commands.
- Add list sorting by score, ping, country, or hostname.
- Add JSON and CSV output formats for the list command.
- Add cache controls with refresh/no-cache flags and cache management commands.
- Improve interactive server selection labels with aligned hostname, country, IP, ping, and score details.
- Add usage examples for filtering, sorting, cache controls, and random filtered connections.

## 0.3.5

- chore: update vendorHash in flake.nix (7948580)
- Refactor codebase (bb88db9)

## 0.3.4

- chore(deps): update dependency go to v1.26.0 (#169) (6550901)
- Update module github.com/olekukonko/tablewriter to v1.1.3 (#171) (c03d27a)
- Update module golang.org/x/net to v0.50.0 (#170) (7da1504)

## 0.3.3

- Update dependency go to v1.25.6 (#167) (ff1d10e)
- Update module golang.org/x/net to v0.49.0 (#168) (9939da1)
- Update module github.com/spf13/afero to v1.15.0 (#143) (9fa908f)
- Update module github.com/spf13/cobra to v1.10.2 (#166) (3f7d49f)
- Update module golang.org/x/net to v0.48.0 (#164) (5ac7d49)

## 0.3.2

- Update dependency go to 1.25 (#156) (1de072b)
- Update module github.com/spf13/cobra to v1.10.1 (#159) (486fc18)
- Update module github.com/stretchr/testify to v1.11.1 (#158) (52cadc8)
- Update module github.com/rs/zerolog to v1.34.0 (#151) (98bb23e)
- Update module github.com/spf13/cobra to v1.9.1 (#147) (552a6e3)
- Update module golang.org/x/net to v0.35.0 (#145) (4bd470b)

## 0.3.1

- Add "386" goarch to .goreleaser.yaml (4c66b19)

## 0.3.0

- Add initial support and docs for Windows (#132) (3e819c5)
