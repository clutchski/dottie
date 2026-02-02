# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Workflow

1. Make a plan
2. Reproduce the error / write a failing test
3. Fix the code
4. Run `make fix test lint`
5. Repeat

See `Makefile` for all available commands.

## Architecture

Dottie is a dotfiles manager CLI. Entry point: `cmd/dottie/main.go` -> `internal/cli/cli.go`.

- **internal/config**: Loads `dottie.yaml`, handles path resolution and ignore patterns
- **internal/link**: Symlink creation with backup/conflict handling
- **internal/status**: Scans HOME for dotfiles, reports link status
- **internal/install**: Package installation (Homebrew/apt)
- **internal/hooks**: Runs pre/post scripts
- **internal/util**: Filesystem helpers

Key behavior: files in `home/` are symlinked to `$HOME` with a dot prefix (`home/vimrc` -> `~/.vimrc`). If a target directory exists as a real directory (not symlink), contents are linked individually.
