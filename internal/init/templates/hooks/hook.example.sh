#!/bin/bash
# Generic hook template
# To enable: cp hooks/hook.example.sh hooks/90-custom.sh
#
# Phases:
#   pre-link  - runs before symlinking (install dependencies here)
#   post-link - runs after symlinking (configure tools here)
#   status    - exit 0 if ok, exit 1 if needs update, exit 2+ if failed

set -euo pipefail
source "$(dirname "$0")/lib.example.sh"

case "$1" in
    pre-link)
        log "add your pre-link setup here"
        ;;
    post-link)
        log "add your post-link setup here"
        ;;
    status)
        # Exit 1 if setup is still needed.
        exit 0
        ;;
esac
