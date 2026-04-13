#!/bin/bash
# mise runtime management hook
# To enable: cp hooks/mise.example.sh hooks/02-mise.sh

set -euo pipefail
source "$(dirname "$0")/lib.example.sh"

case "$1" in
    pre-link)
        if ! has_cmd mise; then
            if is_macos && has_cmd brew; then
                run_or_echo brew install mise
            else
                run_or_echo sh -c "curl https://mise.run | sh"
            fi
        fi

        if has_cmd mise && [[ -f "$DOTTIE_ROOT/mise.toml" ]]; then
            if is_dry_run; then
                log "[dry-run] (cd \"$DOTTIE_ROOT\" && mise install)"
            else
                (cd "$DOTTIE_ROOT" && mise install)
            fi
        fi
        ;;
    status)
        if [[ ! -f "$DOTTIE_ROOT/mise.toml" ]]; then
            echo "missing mise.toml (create one with [tools] entries)"
            exit 1
        fi
        has_cmd mise || exit 1
        ;;
esac
