#!/usr/bin/env bash
# Build agennextd for all supported platforms, archive, and checksum.
# Output -> $OUT (default dist/). Used by the release workflow and for local
# cross-builds. CGO disabled -> static, reproducible, cross-compilable anywhere.
#
#   VERSION=0.1.0 scripts/build-release.sh
set -euo pipefail
VERSION="${VERSION:-0.1.0}"
OUT="${OUT:-dist}"
PLATFORMS="${PLATFORMS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64}"
mkdir -p "$OUT"

for p in $PLATFORMS; do
  os="${p%/*}"; arch="${p#*/}"
  ext=""; [ "$os" = "windows" ] && ext=".exe"
  echo "==> building ${p}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -ldflags="-s -w" -trimpath \
    -o "${OUT}/agennextd${ext}" ./cmd/agennextd
  if [ "$os" = "windows" ]; then
    ( cd "$OUT" && zip -q "agennextd_${VERSION}_${os}_${arch}.zip" "agennextd${ext}" )
  else
    tar -C "$OUT" -czf "${OUT}/agennextd_${VERSION}_${os}_${arch}.tar.gz" "agennextd${ext}"
  fi
  rm -f "${OUT}/agennextd${ext}"
done

( cd "$OUT" && { shasum -a 256 agennextd_"${VERSION}"_* 2>/dev/null || sha256sum agennextd_"${VERSION}"_*; } > checksums.txt )
echo "==> artifacts in ${OUT}:"; ls -1 "${OUT}"/agennextd_"${VERSION}"_*
echo "==> checksums:"; cat "${OUT}/checksums.txt"
