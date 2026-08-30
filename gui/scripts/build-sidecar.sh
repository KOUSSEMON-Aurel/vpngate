#!/usr/bin/env bash
# Builds the Go vpngate binary as a Tauri sidecar.
# The artifact must be named vpngate-<target-triple> so Tauri can resolve
# it from bundle.externalBin. Run before `tauri dev` / `tauri build`.
set -euo pipefail

cd "$(dirname "$0")/../.."

triple="${TAURI_TARGET_TRIPLE:-$(rustc -vV | sed -n 's/^host: //p')}"
mkdir -p gui/src-tauri/binaries

CGO_ENABLED=0 go build -o "gui/src-tauri/binaries/vpngate-${triple}" .
echo "sidecar built: gui/src-tauri/binaries/vpngate-${triple}"