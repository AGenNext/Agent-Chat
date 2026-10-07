#!/bin/sh
# Autonomyx / agennextd installer for macOS (and Linux).
#
#   curl -fsSL https://raw.githubusercontent.com/AGenNextHub/Agent-Chat/main/install/install.sh | sh
#
# Env:
#   VERSION  release version to install (default: 0.1.0)
#   PREFIX   install prefix (default: /usr/local  ->  $PREFIX/bin/agennextd)
#
# Tries a published release binary first; falls back to building from source
# (requires Go). Pure POSIX sh — works with macOS's /bin/sh.
set -eu

REPO="AGenNextHub/Agent-Chat"
BIN="agennextd"
PREFIX="${PREFIX:-/usr/local}"
VERSION="${VERSION:-0.1.0}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)   arch=amd64 ;;
  arm64|aarch64)  arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin|linux) ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

asset="${BIN}_${VERSION}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/v${VERSION}/${asset}"

echo "==> Autonomyx installer  (os=${os} arch=${arch} version=${VERSION})"
if curl -fsSL "$url" -o "${tmp}/${asset}" 2>/dev/null; then
  echo "==> downloaded ${asset}"
  tar -C "$tmp" -xzf "${tmp}/${asset}"
else
  echo "==> no published binary at ${url}; building from source"
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is not installed. Install it (on macOS: brew install go) or wait for a release." >&2
    exit 1
  fi
  if ! GOBIN="$tmp" go install "github.com/${REPO}/cmd/${BIN}@v${VERSION}" 2>/dev/null; then
    echo "==> go install failed; cloning + building"
    git clone --depth 1 "https://github.com/${REPO}.git" "${tmp}/src"
    ( cd "${tmp}/src" && CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o "${tmp}/${BIN}" "./cmd/${BIN}" )
  fi
fi

dest="${PREFIX}/bin"
echo "==> installing to ${dest} (may prompt for sudo)"
if [ -w "$dest" ]; then
  install -m 0755 "${tmp}/${BIN}" "${dest}/${BIN}"
else
  sudo install -m 0755 "${tmp}/${BIN}" "${dest}/${BIN}"
fi

echo "==> installed: ${dest}/${BIN}"
echo "    try:  ${BIN} -demo"
