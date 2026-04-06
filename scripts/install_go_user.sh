#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "This installer supports Linux only." >&2
  exit 1
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

GOOS="linux"
INSTALL_ROOT="${GO_INSTALL_ROOT:-$HOME/.local}"
GOROOT_DIR="$INSTALL_ROOT/go"
GOPATH_DIR="${GOPATH:-$HOME/go}"
TMP_JSON="/tmp/go-dl.json"

LATEST_VERSION="$(curl -fsSL 'https://go.dev/dl/?mode=json' | tee "$TMP_JSON" | grep -oE '"version"[[:space:]]*:[[:space:]]*"go[0-9]+\.[0-9]+(\.[0-9]+)?"' | head -n1 | sed -E 's/.*"go([0-9]+\.[0-9]+(\.[0-9]+)?)".*/\1/')"
if [[ -z "$LATEST_VERSION" ]]; then
  LATEST_VERSION="1.26.1"
fi

TARBALL="go${LATEST_VERSION}.${GOOS}-${GOARCH}.tar.gz"
URL="https://go.dev/dl/${TARBALL}"

cd /tmp
curl -fL -o "$TARBALL" "$URL"

mkdir -p "$INSTALL_ROOT"
rm -rf "$GOROOT_DIR"
tar -C "$INSTALL_ROOT" -xzf "$TARBALL"

BASHRC="$HOME/.bashrc"
if ! grep -qF '### GO ENV ###' "$BASHRC" 2>/dev/null; then
  {
    printf '\n### GO ENV ###\n'
    printf 'export GOROOT="$HOME/.local/go"\n'
    printf 'export GOPATH="$HOME/go"\n'
    printf 'export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"\n'
    printf 'export GOPROXY="https://goproxy.cn,direct"\n'
  } >> "$BASHRC"
fi

export GOROOT="$GOROOT_DIR"
export GOPATH="$GOPATH_DIR"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"

GO_VERSION_OUTPUT="$(go version)"
echo "Installed: $GO_VERSION_OUTPUT"
echo "GOROOT: $GOROOT"
echo "GOPATH: $GOPATH"
echo "Note: run 'source ~/.bashrc' in your current interactive shell."
