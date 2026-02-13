#!/bin/bash
set -e

REPO="clutchski/dottie"
QUIET="${QUIET:-}"

log() {
    [ -z "$QUIET" ] && echo "$@"
}

OS=$(uname -s | tr '[:upper:]' '[:lower:]')

# macOS: install via Homebrew
if [ "$OS" = "darwin" ]; then
    if ! command -v brew &>/dev/null; then
        echo "Error: Homebrew is required on macOS. Install it from https://brew.sh"
        exit 1
    fi
    log ""
    log "==> Installing dottie via Homebrew"
    log ""
    brew install clutchski/tap/dottie
    exit 0
fi

# Linux: download pre-built binary
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$INSTALL_DIR"

case "$OS" in
    linux) OS="Linux" ;;
    *)
        echo "Error: Unsupported OS: $OS"
        exit 1
        ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    amd64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
    aarch64) ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

log ""
log "==> Installing dottie"
log ""
log "    Platform: ${OS}/${ARCH}"

# Get latest version
VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo "Error: Failed to fetch latest version from GitHub"
    exit 1
fi

log "    Version:  ${VERSION}"
log "    Target:   ${INSTALL_DIR}/dottie"

# Download and extract
TARBALL="dottie_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

if ! curl -fsSL "$URL" | tar xz -C "$TMP_DIR"; then
    echo "Error: Failed to download ${URL}"
    exit 1
fi

# Install
mv "$TMP_DIR/dottie" "$INSTALL_DIR/dottie"
