#!/bin/sh
# open-hl7 installer
#
#   curl -fsSL https://raw.githubusercontent.com/rollingventures/open-hl7/main/install.sh | sh
#
# Environment:
#   OPEN_HL7_VERSION   version tag to install (default: latest)
#   OPEN_HL7_BIN_DIR   install directory (default: /usr/local/bin, or ~/.local/bin if not writable)
set -eu

REPO="rollingventures/open-hl7"
BIN="open-hl7"
VERSION="${OPEN_HL7_VERSION:-latest}"

say()  { printf '\033[1;36mopen-hl7\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mopen-hl7\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31mopen-hl7 error:\033[0m %s\n' "$*" >&2; exit 1; }
has()  { command -v "$1" >/dev/null 2>&1; }

fetch() { # fetch URL OUTFILE
  if has curl; then curl -fsSL "$1" -o "$2"
  elif has wget; then wget -q "$1" -O "$2"
  else err "need curl or wget"; fi
}

sha256() { # sha256 FILE -> hash
  if has sha256sum; then sha256sum "$1" | awk '{print $1}'
  elif has shasum; then shasum -a 256 "$1" | awk '{print $1}'
  else echo ""; fi
}

main() {
  has tar || err "tar is required"

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) err "unsupported architecture: $arch" ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *) err "unsupported OS: $os (build from source: go install github.com/$REPO/cmd/hubd@latest)" ;;
  esac

  if [ "$VERSION" = "latest" ]; then
    base="https://github.com/$REPO/releases/latest/download"
  else
    base="https://github.com/$REPO/releases/download/$VERSION"
  fi
  asset="${BIN}_${os}_${arch}.tar.gz"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT INT TERM

  say "downloading $asset ($VERSION) ..."
  if ! fetch "$base/$asset" "$tmp/$asset"; then
    err "could not download $base/$asset
  No release published yet? Install from source instead:
      go install github.com/$REPO/cmd/hubd@latest"
  fi

  # Verify checksum if the release publishes one.
  if fetch "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
    want="$(grep " $asset\$" "$tmp/checksums.txt" 2>/dev/null | awk '{print $1}' || true)"
    got="$(sha256 "$tmp/$asset")"
    if [ -n "$want" ] && [ -n "$got" ]; then
      [ "$want" = "$got" ] || err "checksum mismatch for $asset"
      say "checksum verified"
    else
      warn "skipping checksum (no tool or entry found)"
    fi
  fi

  tar -xzf "$tmp/$asset" -C "$tmp"
  [ -f "$tmp/$BIN" ] || err "archive did not contain $BIN"
  chmod +x "$tmp/$BIN"

  # Choose an install dir we can write to.
  bindir="${OPEN_HL7_BIN_DIR:-/usr/local/bin}"
  if [ ! -d "$bindir" ] || [ ! -w "$bindir" ]; then
    if [ -z "${OPEN_HL7_BIN_DIR:-}" ] && has sudo && [ -d /usr/local/bin ]; then
      say "installing to /usr/local/bin (sudo)"
      sudo install -m 0755 "$tmp/$BIN" "/usr/local/bin/$BIN"
      finish "/usr/local/bin/$BIN"
      return
    fi
    bindir="$HOME/.local/bin"
    mkdir -p "$bindir"
  fi
  install -m 0755 "$tmp/$BIN" "$bindir/$BIN"
  finish "$bindir/$BIN"
}

finish() { # finish DEST
  dest="$1"
  say "installed -> $dest"
  case ":$PATH:" in
    *":$(dirname "$dest"):"*) ;;
    *) warn "add $(dirname "$dest") to your PATH" ;;
  esac
  say "run: $BIN -h"
}

main "$@"
