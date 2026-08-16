#!/bin/sh
#
# vpngate installer
#
#   curl -fsSL https://raw.githubusercontent.com/KOUSSEMON-Aurel/vpngate/main/install.sh | bash
#
# Installs the vpngate binary (GitHub release, falling back to go install),
# plus its runtime dependencies where a supported package manager exists:
#   - openvpn        (required for the vpngate/vpnbook relays)
#   - wireguard-tools (required for the wgcf Cloudflare WARP backend)
#   - wgcf           (optional, WARP backend; go install when missing)
#   - warp-cli       (optional, WARP fallback backend)
#
# Environment overrides:
#   VPNGATE_VERSION      pinned release tag (default: latest)
#   VPNGATE_INSTALL_DIR  binary destination (default: /usr/local/bin)

REPO="KOUSSEMON-Aurel/vpngate"
RELEASE="${VPNGATE_VERSION:-}"
DEST="${VPNGATE_INSTALL_DIR:-/usr/local/bin}"

say()  { printf '\033[1;32m[install]\033[0m %s\n' "$1"; }
warn() { printf '\033[1;33m[install]\033[0m %s\n' "$1"; }
die()  { printf '\033[1;31m[install]\033[0m %s\n' "$1" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1; }

writable() { [ -w "$1" ]; }

run_privileged() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif need sudo; then
    sudo "$@"
  else
    return 127
  fi
}

install_binary() {
  say "resolving latest release"
  if [ -z "$RELEASE" ]; then
    RELEASE=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
  fi

  if [ -n "$RELEASE" ] && [ "$RELEASE" != "main" ]; then
    os=$(uname -s)
    case "$os" in
      Linux)  os="Linux" ;;
      Darwin) os="Darwin" ;;
      *) warn "unsupported OS for release download: $os" && RELEASE="main" ;;
    esac
    arch=$(uname -m)
    case "$arch" in
      x86_64|amd64)        arch="x86_64" ;;
      aarch64|arm64)       arch="arm64" ;;
      armv7l|armv7|armhf)  arch="armv7" ;;
      i386|i686)           arch="386" ;;
      *) warn "unsupported architecture for release download: $arch" && RELEASE="main" ;;
    esac
  fi

  if [ -n "$RELEASE" ] && [ "$RELEASE" != "main" ]; then
    version=${RELEASE#v}
    url="https://github.com/$REPO/releases/download/$RELEASE/vpngate_${version}_${os}_${arch}.tar.gz"
    say "downloading $url"
    tmp=$(mktemp -d)
    if ! curl -fsSL "$url" -o "$tmp/vpngate.tar.gz"; then
      warn "release download failed; falling back to go install"
      RELEASE="main"
      rm -rf "$tmp"
    fi
    if [ "$RELEASE" != "main" ]; then
      tar -xzf "$tmp/vpngate.tar.gz" -C "$tmp"
      rm -f "$tmp/vpngate.tar.gz"
      bin=$(find "$tmp" -type f -name vpngate | head -n 1)
      [ -n "$bin" ] || die "release archive did not contain the vpngate binary"
      install -m 0755 "$bin" "$DEST/vpngate"
      rm -rf "$tmp"
      say "installed $DEST/vpngate ($RELEASE)"
      return 0
    fi
  fi

  need go || die "no release found and no Go toolchain available; install Go (https://go.dev/dl) or download a release manually"
  say "installing from source with go install"
  go install "github.com/$REPO@latest"
  bin=$(command -v vpngate || true)
  if [ -z "$bin" ]; then
    bin="$(go env GOPATH)/bin/vpngate"
    [ -f "$bin" ] || die "go install finished but $bin was not produced"
  fi
  if [ "$bin" = "$DEST/vpngate" ]; then
    say "installed $DEST/vpngate"
  elif writable "$DEST"; then
    install -m 0755 "$bin" "$DEST/vpngate"
    say "installed $DEST/vpngate"
  else
    say "installed $bin (add it to your PATH)"
  fi
}

install_pkg() {
  if need apt-get; then
    run_privileged apt-get update -qq && run_privileged apt-get install -y -qq "$@"
  elif need dnf; then
    run_privileged dnf install -y "$@"
  elif need pacman; then
    run_privileged pacman -S --noconfirm "$@"
  elif need brew; then
    brew install "$@"
  else
    return 1
  fi
}

install_deps() {
  pkgs=""
  need openvpn || pkgs="$pkgs openvpn"
  need wg-quick || pkgs="$pkgs wireguard-tools"
  if [ -n "$pkgs" ]; then
    say "installing openvpn and wireguard-tools"
    if install_pkg $pkgs; then
      say "runtime dependencies installed"
    else
      warn "could not install runtime dependencies; install openvpn and wireguard-tools manually"
    fi
  else
    say "openvpn and wireguard-tools already installed"
  fi

  if need wgcf; then
    say "wgcf already installed"
  elif need go; then
    say "installing wgcf (Cloudflare WARP backend)"
    go install github.com/ViRb3/wgcf@latest || warn "wgcf install failed; WARP via wgcf will be unavailable"
  else
    warn "wgcf not installed and no Go toolchain to build it; the wgcf WARP backend needs it"
  fi

  if need warp-cli; then
    say "warp-cli already installed"
  else
    warn "warp-cli not found; it is only needed for the fallback WARP backend (warp-cli --accept-tos)"
  fi
}

main() {
  install_binary
  install_deps
  say "done. try: vpngate list"
  say "WARP is available via: vpngate warp (needs wgcf + wg-quick, or warp-cli)"
}

main