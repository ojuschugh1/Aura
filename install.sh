#!/usr/bin/env bash
# install.sh — download and install the aura binary for the current OS/arch.
set -euo pipefail

REPO="ojuschugh1/aura"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  echo "${os}-${arch}"
}

PLATFORM="$(detect_platform)"
EXT=""
if [[ "$PLATFORM" == windows* ]]; then EXT=".exe"; fi

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
fi

URL="https://github.com/${REPO}/releases/download/${VERSION}/aura-${PLATFORM}${EXT}"
TMP="$(mktemp)"

echo "Downloading aura ${VERSION} for ${PLATFORM}..."
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"

# Verify checksum if a .sha256 file is available.
SHA_URL="${URL}.sha256"
if curl -fsSL "$SHA_URL" -o "${TMP}.sha256" 2>/dev/null; then
  expected="$(awk '{print $1}' "${TMP}.sha256")"
  actual="$(sha256sum "$TMP" | awk '{print $1}')"
  if [[ "$expected" != "$actual" ]]; then
    echo "checksum mismatch: expected $expected, got $actual" >&2
    rm -f "$TMP" "${TMP}.sha256"
    exit 1
  fi
  echo "checksum verified"
  rm -f "${TMP}.sha256"
fi

install -m 0755 "$TMP" "${INSTALL_DIR}/aura${EXT}"
rm -f "$TMP"
echo "aura installed to ${INSTALL_DIR}/aura${EXT}"
