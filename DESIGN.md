# Dottie: Simple Dotfiles Manager (Go)

## Overview

A Go-based dotfiles manager with a clean CLI. Distributed as a single binary.

## CLI Design

```bash
dottie init            # Set up a new dotfiles repo
dottie link            # Symlink dotfiles into place
dottie install         # Install tools (homebrew, apt-get packages)
dottie run             # Do everything: install + link
dottie status          # Show what's linked/installed

# Bootstrap a new machine:
curl -fsSL https://raw.githubusercontent.com/.../bootstrap.sh | bash
```

## Project Structure (this repo)

```
dottie/
├── cmd/
│   └── dottie/
│       └── main.go         # Entry point
├── internal/
│   ├── cli/
│   │   └── cli.go          # Command parsing, subcommand routing
│   ├── config/
│   │   └── config.go       # Load/parse dotfiles repo config
│   ├── link/
│   │   └── link.go         # Symlink logic
│   ├── install/
│   │   └── install.go      # Package installation (brew/apt)
│   ├── status/
│   │   └── status.go       # Status and diff commands
│   ├── init/
│   │   └── init.go         # Scaffold new dotfiles repo
│   ├── hooks/
│   │   └── hooks.go        # Hook execution
│   └── util/
│       ├── fs.go           # File system helpers
│       ├── os.go           # OS detection
│       └── backup.go       # Backup logic
├── scripts/
│   └── bootstrap.sh        # curl|bash installer (downloads binary)
├── go.mod
├── go.sum
├── Makefile                # Build targets
└── README.md
```

## User's Dotfiles Repo Structure

When a user runs `dottie init`, it creates:

```
~/dotfiles/                 # User's dotfiles repo
├── .dottie.yaml            # Config file
├── vimrc                   # Dotfiles at root (no dot prefix)
├── bashrc
├── gitconfig
├── config/                 # Directories also without dot
│   └── nvim/
│       └── init.lua
├── hooks/                  # Optional hook scripts (directories)
│   ├── pre-install/
│   │   └── .gitkeep
│   ├── post-install/
│   │   └── .gitkeep
│   ├── pre-link/
│   │   └── .gitkeep
│   └── post-link/
│       └── .gitkeep
├── deps/                   # Package manager files
│   ├── Brewfile            # macOS packages
│   └── apt.txt             # Linux packages
└── .gitignore
```

Files are linked with a dot prepended: `vimrc` -> `~/.vimrc`, `config/nvim/` -> `~/.config/nvim/`

### Hooks

Hook directories allow multiple scripts per stage:
- `hooks/pre-install/` - scripts run before package installation
- `hooks/post-install/` - scripts run after package installation
- `hooks/pre-link/` - scripts run before linking
- `hooks/post-link/` - scripts run after linking

Scripts within each directory run in alphabetical order (use numeric prefixes like `01-setup.sh` for ordering).
All hooks are optional - directories with only `.gitkeep` are skipped.

## Subcommands

### `dottie init [dir]`
Scaffold a new dotfiles repo:
```bash
dottie init                 # Current directory
dottie init ~/dotfiles      # Specified directory
```

### `dottie link`
```bash
dottie link                   # Link all (backup existing files first)
dottie link -n, --dry-run     # Preview changes
dottie link -f, --force       # No backup, just overwrite
```

### `dottie install`
```bash
dottie install              # Install packages for current OS
dottie install -n           # Dry-run
```

### `dottie run`
```bash
dottie run                  # install + link
dottie run -n               # Dry-run
```

### `dottie status`
```
Dotfiles:
  [linked]    .bashrc
  [linked]    .gitconfig
  [missing]   .vimrc
  [conflict]  .zshrc         (exists, not a symlink)

Packages:
  [installed]     git
  [not installed] ripgrep
```

## Bootstrap Flow

```bash
curl -fsSL https://raw.githubusercontent.com/clutchski/dottie/main/scripts/bootstrap.sh | bash
```

`bootstrap.sh`:
1. Detect OS and architecture
2. Download correct binary from GitHub releases
3. Install to `/usr/local/bin/dottie` (or `~/.local/bin`)
4. Clone user's dotfiles repo (if URL provided)
5. Run `dottie run`

## Config File (.dottie.yaml)

```yaml
# Where dotfiles are in the repo (default: . = repo root)
source_dir: .

# Where to link/copy them (default: $HOME)
target_dir: ~

# Prepend . to filenames when linking (default: true)
# e.g., vimrc -> .vimrc, config/nvim -> .config/nvim
add_dot: true

# Where to store backups of existing files
backup_dir: ~/.dottie.backup

# What to do on conflict: backup | skip | overwrite
conflict: backup

# Files/directories to ignore (always ignores .dottie.yaml, .git, hooks/, deps/)
ignore:
  - README.md
  - LICENSE

# Deps directory (for package files)
deps_dir: deps

# Hooks directory (for pre/post scripts)
hooks_dir: hooks
```

## Implementation Phases

### Phase 1: Core CLI
1. Set up Go module, basic project structure
2. `cmd/dottie/main.go` - entry point with cobra or basic flag parsing
3. `internal/util/` - OS detection, file helpers, backup logic
4. `internal/cli/` - subcommand routing

### Phase 2: Link & Config
5. `internal/link/` - symlink logic with dry-run, backup
6. `internal/config/` - load .dottie.yaml

### Phase 3: Init & Install
8. `internal/init/` - scaffold new dotfiles repo
9. `internal/install/` - brew bundle, apt install
10. `internal/hooks/` - hook execution logic

### Phase 4: Status
10. `internal/status/` - status display

### Phase 5: Bootstrap & Release
11. `scripts/bootstrap.sh` - download binary, run setup
12. Makefile with build targets for darwin/linux amd64/arm64
13. GitHub Actions for releases (optional, or use goreleaser)

## Build & Release

```makefile
# Makefile targets
build:           # Build for current OS
build-all:       # Build for darwin-amd64, darwin-arm64, linux-amd64, linux-arm64
install:         # Install to /usr/local/bin
```

## Verification

1. `go build ./cmd/dottie` - builds successfully
2. `./dottie init /tmp/test` - creates scaffold
3. `./dottie link -n` - shows expected symlinks
4. `./dottie link` - creates symlinks, backs up existing
5. `./dottie status` - shows linked files
6. `./dottie install -n` - shows packages
9. Test `curl | bash` with built binary on GitHub
