#!/bin/bash
# apt package management hook (Debian/Ubuntu)
# To enable: cp hooks/apt.example.sh hooks/01-apt.sh
APTFILE="$DOTTIE_ROOT/Aptfile"

set -euo pipefail
source "$(dirname "$0")/lib.example.sh"

read_aptfile() {
    grep -v '^\s*#' "$APTFILE" | sed '/^\s*$/d'
}

case "$1" in
    pre-link)
        is_linux || exit 0
        has_cmd apt-get || exit 0
        [[ -f "$APTFILE" ]] || exit 0

        mapfile -t packages < <(read_aptfile)
        [[ ${#packages[@]} -gt 0 ]] || exit 0

        run_or_echo sudo apt-get update
        run_or_echo sudo apt-get install -y "${packages[@]}"
        ;;
    status)
        is_linux || exit 0
        has_cmd apt-get || exit 0
        if [[ ! -f "$APTFILE" ]]; then
            echo "missing Aptfile (create it from Aptfile.example)"
            exit 1
        fi

        mapfile -t packages < <(read_aptfile)
        if [[ ${#packages[@]} -eq 0 ]]; then
            echo "Aptfile is empty; add package names"
            exit 1
        fi

        for pkg in "${packages[@]}"; do
            dpkg -s "$pkg" &>/dev/null || {
                echo "apt package missing: $pkg"
                exit 1
            }
        done
        ;;
esac
