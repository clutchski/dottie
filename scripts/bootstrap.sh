#!/bin/bash
set -e

# User must set DOTFILES_REPO before calling this script
if [ -z "$DOTFILES_REPO" ]; then
    echo "Error: DOTFILES_REPO must be set"
    exit 1
fi

DOTFILES_DIR="${DOTFILES_DIR:-$HOME/.dotfiles}"
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

OS=$(uname -s | tr '[:upper:]' '[:lower:]')

# macOS: install dottie via Homebrew
if [ "$OS" = "darwin" ]; then
    if ! command -v brew &>/dev/null; then
        echo "    Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi
    echo "    Installing dottie via Homebrew..."
    brew install clutchski/tap/dottie
else
    # Linux: download pre-built binary
    INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
    mkdir -p "$INSTALL_DIR"

    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
    esac

    VERSION=$(curl -sL "https://api.github.com/repos/${DOTTIE_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Error: Failed to fetch dottie version"
        exit 1
    fi

    echo "    Dottie:    $VERSION"
    echo ""

    TARBALL="dottie_${VERSION#v}_Linux_${ARCH}.tar.gz"
    URL="https://github.com/${DOTTIE_REPO}/releases/download/${VERSION}/${TARBALL}"

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    if ! curl -fsSL "$URL" | tar xz -C "$TMP_DIR"; then
        echo "Error: Failed to download dottie"
        exit 1
    fi

    mv "$TMP_DIR/dottie" "$INSTALL_DIR/dottie"
fi

# Run dottie
cd "$DOTFILES_DIR"
dottie run

echo ""
echo "    Done. Your dotfiles are ready."
echo ""
echo "    Next time: cd $DOTFILES_DIR && dottie run"
echo ""
