#!/bin/bash
set -e

REPO="clutchski/dottie"
if [ -z "$INSTALL_DIR" ]; then
    if [ -d "$HOME/.local/bin" ]; then
        INSTALL_DIR="$HOME/.local/bin"
    else
        INSTALL_DIR="/usr/local/bin"
    fi
fi
QUIET="${QUIET:-}"

log() {
    [ -z "$QUIET" ] && echo "$@"
}

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin) OS="Darwin" ;;
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
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/dottie" "$INSTALL_DIR/dottie"
else
    sudo mv "$TMP_DIR/dottie" "$INSTALL_DIR/dottie"
fi
