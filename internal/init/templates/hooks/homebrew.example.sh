#!/bin/bash
# Homebrew package management hook
# To enable: cp hooks/homebrew.example.sh hooks/01-homebrew.sh
BREWFILE="$DOTTIE_ROOT/Brewfile"

set -euo pipefail
source "$(dirname "$0")/lib.example.sh"

case "$1" in
    pre-link)
        # Homebrew is macOS-first. Skip silently elsewhere.
        is_macos || exit 0

        # Install Homebrew if missing (macOS only)
        if ! has_cmd brew; then
            run_or_echo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        fi

        # Install packages from Brewfile
        if [[ -f "$BREWFILE" ]] && has_cmd brew; then
            if is_dry_run; then
                log "[dry-run] brew bundle --file=$BREWFILE"
            else
                brew bundle check --file="$BREWFILE" &>/dev/null || brew bundle --file="$BREWFILE"
            fi
        fi
        ;;
    status)
        is_macos || exit 0
        has_cmd brew || exit 1

        # Exit 1 = needs update (manifest/packages missing), exit 0 = ok
        if [[ ! -f "$BREWFILE" ]]; then
            echo "missing Brewfile (create it from Brewfile.example)"
            exit 1
        fi
        brew bundle check --file="$BREWFILE" &>/dev/null || {
            echo "Brewfile packages need install/update"
            exit 1
        }
        ;;
esac
