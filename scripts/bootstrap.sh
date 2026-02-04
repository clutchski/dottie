#!/bin/bash
set -e

# User must set DOTFILES_REPO before calling this script
if [ -z "$DOTFILES_REPO" ]; then
    echo "Error: DOTFILES_REPO must be set"
    exit 1
fi

DOTFILES_DIR="${DOTFILES_DIR:-$HOME/.dotfiles}"
if [ -z "$INSTALL_DIR" ]; then
    if [ -d "$HOME/.local/bin" ]; then
        INSTALL_DIR="$HOME/.local/bin"
    else
        INSTALL_DIR="/usr/local/bin"
    fi
fi
DOTTIE_REPO="clutchski/dottie"

echo ""
echo "    .___      __    __  .__        "
echo "  __| _/_____/  |__/  |_|__| ____  "
echo " / __ |/  _ \   __\   __\  |/ __ \ "
echo "/ /_/ (  <_> )  |  |  | |  \  ___/ "
echo "\____ |\____/|__|  |__| |__|\___  >"
echo "     \/                         \/ "
echo ""
echo "        Bootstrapping your dotfiles..."
echo ""

echo "    Dotfiles:  $DOTFILES_REPO"
echo "    Target:    $DOTFILES_DIR"
echo "    Platform:  $(uname -s)/$(uname -m)"

# Clone or update dotfiles
if [ -d "$DOTFILES_DIR" ]; then
    git -C "$DOTFILES_DIR" pull --quiet
else
    git clone --quiet "https://github.com/$DOTFILES_REPO" "$DOTFILES_DIR"
fi

# Detect OS and architecture
OS=$(uname -s)
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# Get latest dottie version
VERSION=$(curl -sL "https://api.github.com/repos/${DOTTIE_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Error: Failed to fetch dottie version"
    exit 1
fi

echo "    Dottie:    $VERSION"
echo ""

# Download and install dottie
TARBALL="dottie_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${DOTTIE_REPO}/releases/download/${VERSION}/${TARBALL}"

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

if ! curl -fsSL "$URL" | tar xz -C "$TMP_DIR"; then
    echo "Error: Failed to download dottie"
    exit 1
fi

if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/dottie" "$INSTALL_DIR/dottie"
else
    sudo mv "$TMP_DIR/dottie" "$INSTALL_DIR/dottie"
fi

# Run dottie
cd "$DOTFILES_DIR"
dottie run

echo ""
echo "    Done. Your dotfiles are ready."
echo ""
echo "    Next time: cd $DOTFILES_DIR && dottie run"
echo ""
