#!/bin/bash
#
# Dottie Bootstrap Script
# Downloads and installs dottie, then optionally sets up dotfiles
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/clutchski/dottie/main/scripts/bootstrap.sh | bash
#   curl -fsSL ... | bash -s -- --repo https://github.com/user/dotfiles
#

set -e

REPO_URL=""
INSTALL_DIR="/usr/local/bin"
GITHUB_REPO="clutchski/dottie"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --repo)
            REPO_URL="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Darwin)
            echo "darwin"
            ;;
        Linux)
            echo "linux"
            ;;
        *)
            echo "Unsupported OS: $(uname -s)" >&2
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        *)
            echo "Unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac
}

# Get latest release version
get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install dottie
install_dottie() {
    local os="$1"
    local arch="$2"
    local version="$3"

    local filename="dottie_${version#v}_${os}_${arch}.tar.gz"
    local url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${filename}"

    echo "Downloading dottie ${version} for ${os}/${arch}..."

    local tmpdir
    tmpdir=$(mktemp -d)
    trap "rm -rf ${tmpdir}" EXIT

    curl -fsSL "${url}" -o "${tmpdir}/dottie.tar.gz"
    tar -xzf "${tmpdir}/dottie.tar.gz" -C "${tmpdir}"

    # Install binary
    if [[ -w "${INSTALL_DIR}" ]]; then
        cp "${tmpdir}/dottie" "${INSTALL_DIR}/dottie"
    else
        echo "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo cp "${tmpdir}/dottie" "${INSTALL_DIR}/dottie"
    fi

    chmod +x "${INSTALL_DIR}/dottie"

    echo "Installed dottie to ${INSTALL_DIR}/dottie"
}

# Clone dotfiles repo
clone_dotfiles() {
    local repo_url="$1"
    local dotfiles_dir="${HOME}/dotfiles"

    if [[ -d "${dotfiles_dir}" ]]; then
        echo "Dotfiles directory already exists: ${dotfiles_dir}"
        return 0
    fi

    echo "Cloning dotfiles from ${repo_url}..."
    git clone "${repo_url}" "${dotfiles_dir}"
}

# Main
main() {
    echo "==> Detecting system..."
    local os
    os=$(detect_os)
    local arch
    arch=$(detect_arch)
    echo "    OS: ${os}, Arch: ${arch}"

    echo "==> Getting latest version..."
    local version
    version=$(get_latest_version)
    echo "    Version: ${version}"

    echo "==> Installing dottie..."
    install_dottie "${os}" "${arch}" "${version}"

    # Verify installation
    if ! command -v dottie &> /dev/null; then
        export PATH="${INSTALL_DIR}:${PATH}"
    fi

    echo "==> Verifying installation..."
    dottie version

    # Clone and setup dotfiles if repo URL provided
    if [[ -n "${REPO_URL}" ]]; then
        echo "==> Setting up dotfiles..."
        clone_dotfiles "${REPO_URL}"

        cd "${HOME}/dotfiles"
        dottie run
    fi

    echo ""
    echo "Done! Dottie is installed."
    if [[ -z "${REPO_URL}" ]]; then
        echo ""
        echo "To set up your dotfiles:"
        echo "  1. cd ~/dotfiles"
        echo "  2. dottie run"
    fi
}

main "$@"
