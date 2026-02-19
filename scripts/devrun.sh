#!/usr/bin/env bash
# Build dottie from source and run it against a dev dotfiles repo.
set -euo pipefail

DOTTIE_SRC="$(cd "$(dirname "$0")/.." && pwd)"

# Load .env if present
if [ -f "$DOTTIE_SRC/.env" ]; then
  set -a
  source "$DOTTIE_SRC/.env"
  set +a
fi

DOTTIE_DEV_DOTFILES_DIR="${DOTTIE_DEV_DOTFILES_DIR:?DOTTIE_DEV_DOTFILES_DIR must be set (in .env or environment)}"

if [ ! -d "$DOTTIE_DEV_DOTFILES_DIR" ]; then
  echo "Error: dotfiles directory not found: $DOTTIE_DEV_DOTFILES_DIR" >&2
  exit 1
fi

go build -o "$DOTTIE_SRC/dottie" "$DOTTIE_SRC/cmd/dottie"

cd "$DOTTIE_DEV_DOTFILES_DIR"
exec "$DOTTIE_SRC/dottie" "$@"
