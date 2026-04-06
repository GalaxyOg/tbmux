#!/usr/bin/env bash
set -euo pipefail

# Install prebuilt tbmux binary from GitHub Releases.
# No Go toolchain required.
#
# Env vars:
#   TBMUX_REPO=GalaxyOg/tbmux
#   TBMUX_VERSION=latest | v0.1.0
#   TBMUX_PREFIX=$HOME/.local

REPO="${TBMUX_REPO:-GalaxyOg/tbmux}"
VERSION="${TBMUX_VERSION:-latest}"
PREFIX="${TBMUX_PREFIX:-$HOME/.local}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing command: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "当前安装脚本仅支持 Linux" >&2
  exit 1
fi

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "不支持的架构: $ARCH_RAW" >&2
    exit 1
    ;;
esac

if [[ "$VERSION" == "latest" ]]; then
  TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -m1 -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' | sed -E 's/.*"([^"]+)"/\1/')"
  if [[ -z "$TAG" ]]; then
    echo "获取 latest release 失败，请确认仓库已发布 release" >&2
    exit 1
  fi
else
  TAG="$VERSION"
fi

VER_NO_V="${TAG#v}"
ASSET="tbmux_${VER_NO_V}_linux_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

WORKDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

ARCHIVE="$WORKDIR/$ASSET"

printf '[tbmux] repo: %s\n' "$REPO"
printf '[tbmux] tag: %s\n' "$TAG"
printf '[tbmux] asset: %s\n' "$ASSET"

curl -fL "$URL" -o "$ARCHIVE"

tar -xzf "$ARCHIVE" -C "$WORKDIR"

if [[ ! -f "$WORKDIR/tbmux" ]]; then
  echo "解压后未找到 tbmux 可执行文件" >&2
  exit 1
fi

mkdir -p "$PREFIX/bin"
install -m 0755 "$WORKDIR/tbmux" "$PREFIX/bin/tbmux"

printf '[tbmux] installed to: %s\n' "$PREFIX/bin/tbmux"

if ! command -v tbmux >/dev/null 2>&1; then
  echo "请确保 PATH 包含 $PREFIX/bin" >&2
fi

echo "[tbmux] version: $($PREFIX/bin/tbmux version)"
