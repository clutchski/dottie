#!/bin/bash
# Shared hook helpers. Source from hook scripts:
#   source "$(dirname "$0")/lib.example.sh"

log() {
    local name="${DOTTIE_HOOK_NAME:-hook}"
    printf '[%s] %s\n' "$name" "$*"
}

is_macos() {
    [[ "$(uname -s)" == "Darwin" ]]
}

is_linux() {
    [[ "$(uname -s)" == "Linux" ]]
}

has_cmd() {
    command -v "$1" &>/dev/null
}

is_dry_run() {
    [[ "${DOTTIE_DRY_RUN:-false}" == "true" ]]
}

run_or_echo() {
    if is_dry_run; then
        log "[dry-run] $*"
    else
        "$@"
    fi
}
