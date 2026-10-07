#!/usr/bin/env bash
# Build macOS binaries for agennextd (darwin arm64 + amd64), tar them, and
# write SHA-256 checksums. Output goes to $OUT (default: dist/).
# These are the assets the Homebrew formula and install.sh reference from a
# GitHub release. CGO disabled -> static, reproducible, cross-compilable anywhere.
set -euo pipefail
VERSION="${VERSION:-0.1.0}"
OUT="${OUT:-dist}"
mkdir -p "$OUT"
for arch in arm64 amd64; do
  echo "==> building darwin/${arch}"
  CGO_ENABLED=0 GOOS=darwin GOARCH="${arch}" go build -ldflags="-s -w" -trimpath \
    -o "${OUT}/agennextd" ./cmd/agennextd
  tar -C "${OUT}" -czf "${OUT}/agennextd_${VERSION}_darwin_${arch}.tar.gz" agennextd
  rm -f "${OUT}/agennextd"
done
( cd "${OUT}" && { shasum -a 256 agennextd_*_darwin_*.tar.gz 2>/dev/null || sha256sum agennextd_*_darwin_*.tar.gz; } > checksums-darwin.txt )
echo "==> artifacts in ${OUT}:"; ls -1 "${OUT}"/agennextd_*_darwin_*.tar.gz
echo "==> checksums:"; cat "${OUT}/checksums-darwin.txt"
