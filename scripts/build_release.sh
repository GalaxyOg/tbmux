#!/usr/bin/env bash
set -euo pipefail

# Build release archives for tbmux.
# Usage:
#   VERSION=v0.1.0 bash scripts/build_release.sh
# Output:
#   dist/tbmux_<version>_linux_amd64.tar.gz
#   dist/tbmux_<version>_linux_arm64.tar.gz
#   dist/sha256sums.txt

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v go >/dev/null 2>&1; then
  echo "go not found" >&2
  exit 1
fi

VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
fi

VERSION_NO_V="${VERSION#v}"
DIST_DIR="$ROOT_DIR/dist"
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

TARGETS=(
  "linux/amd64"
  "linux/arm64"
)

for target in "${TARGETS[@]}"; do
  GOOS="${target%/*}"
  GOARCH="${target#*/}"

  WORK_DIR="$DIST_DIR/tbmux_${VERSION_NO_V}_${GOOS}_${GOARCH}"
  mkdir -p "$WORK_DIR"

  echo "[build] ${GOOS}/${GOARCH}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o "$WORK_DIR/tbmux" ./cmd/tbmux

  tar -C "$WORK_DIR" -czf "$DIST_DIR/tbmux_${VERSION_NO_V}_${GOOS}_${GOARCH}.tar.gz" tbmux
  rm -rf "$WORK_DIR"
done

(
  cd "$DIST_DIR"
  sha256sum tbmux_*.tar.gz > sha256sums.txt
)

echo "[done] artifacts in $DIST_DIR"
ls -lh "$DIST_DIR"
