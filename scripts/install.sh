#!/bin/sh
# sb-fox installer — downloads the latest release for your OS/arch and
# installs the binary. Usage:
#   curl -fsSL https://raw.githubusercontent.com/mora1n/sb-fox/main/scripts/install.sh | sh
# Env:
#   SB_FOX_VERSION=v0.1.0   pin a specific version (default: latest)
#   SB_FOX_INSTALL_DIR=...  install location (default: /usr/local/bin, or ~/.local/bin without root)
#   SB_FOX_DATA_DIR=...     template seed data location (default: /var/lib/sb-fox for root, ~/.local/share/sb-fox otherwise)
set -eu

REPO="mora1n/sb-fox"
BINARY="sb-fox"

info() { printf '\033[1;34m==>\033[0m %s\n' "$1"; }
err()  { printf '\033[1;31merror:\033[0m %s\n' "$1" >&2; exit 1; }

# --- detect os/arch ---
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux|darwin) ;;
  *) err "unsupported OS: $os (use the Windows zip from the Releases page)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) err "unsupported arch: $arch" ;;
esac

# --- resolve version ---
version="${SB_FOX_VERSION:-}"
if [ -z "$version" ]; then
  info "resolving latest release"
  version="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | cut -d'"' -f4)"
  [ -n "$version" ] || err "could not determine latest version; set SB_FOX_VERSION"
fi

# --- download + verify ---
archive="${BINARY}-${os}-${arch}-${version}.tar.gz"
base="https://github.com/${REPO}/releases/download/${version}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

info "downloading ${archive}"
curl -fsSL "${base}/${archive}" -o "${tmp}/${archive}"

if curl -fsSL "${base}/SHA256SUMS" -o "${tmp}/SHA256SUMS" 2>/dev/null; then
  info "verifying checksum"
  ( cd "$tmp" && grep " ${archive}\$" SHA256SUMS | sha256sum -c - ) \
    || err "checksum verification failed"
else
  info "SHA256SUMS not found — skipping verification"
fi

tar -C "$tmp" -xzf "${tmp}/${archive}"
archive_root="${tmp}/${BINARY}-${os}-${arch}-${version}"

# --- install ---
dir="${SB_FOX_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ "$(id -u)" -eq 0 ]; then dir="/usr/local/bin"; else dir="$HOME/.local/bin"; fi
fi
mkdir -p "$dir"
install -m 0755 "${archive_root}/${BINARY}" "${dir}/${BINARY}"

data_dir="${SB_FOX_DATA_DIR:-}"
if [ -z "$data_dir" ]; then
  if [ "$(id -u)" -eq 0 ]; then data_dir="/var/lib/sb-fox"; else data_dir="$HOME/.local/share/sb-fox"; fi
fi
mkdir -p "${data_dir}/templates"
cp -R "${archive_root}/data/templates/." "${data_dir}/templates/"

info "installed ${BINARY} ${version} to ${dir}/${BINARY}"
info "installed seed templates to ${data_dir}/templates"
info "next steps:"
printf '  %s\n' "enable daemon: sb-fox --daemon"
printf '  %s\n' "restart daemon: sb-fox --daemon restart"
printf '  %s\n' "open panel:   http://127.0.0.1:7878"
printf '  %s\n' "logs:         journalctl -u sb-fox -f"
printf '  %s\n' "registration: sb-fox --reg on|off"
printf '  %s\n' "foreground:   sb-fox --addr 127.0.0.1:7879"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) printf '\033[1;33mnote:\033[0m %s is not in your PATH; add it or move the binary.\n' "$dir" ;;
esac
"${dir}/${BINARY}" --version 2>/dev/null || true
