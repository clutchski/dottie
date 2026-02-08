# dottie

A simple dotfiles manager for macOS and Linux. Keep your dotfiles in a git repo, sync them across machines, and use hooks to  basic system setup with tools like homebrew, apt or mise.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/clutchski/dottie/main/scripts/install.sh | bash
```

This auto-detects your OS and architecture and installs the latest release to `~/.local/bin`. Or install a [recent release](https://github.com/clutchski/dottie/releases) manually.

## Quick Start

### 1. Initialize a Dotfiles Repo

Create a git repo to store your configuration files:

```bash
mkdir ~/dotfiles && cd ~/dotfiles
git init
dottie init
git add
git commit -m "first commit" && git push
```

This creates the repo structure: `home/` for dotfiles, `hooks/` for scripts, and `dottie.yaml` for config.

### 2. Add a Dotfile

Dotfiles are configuration files for your tools (shell, editor, git, etc.). Copy them into `home/` without the leading dot:

```bash
cp ~/.zshrc ~/dotfiles/home/zshrc
```

### 3. Add a Hook (Optional)

Hooks run custom scripts for tasks beyond symlinking: installing packages, setting up plugins, configuring tools. Create one from the template:

```bash
cp hooks/hook.example.sh hooks/01-setup.sh
```

Edit it to run in `pre-link` (before symlinking), `post-link` (after), or `status` (health checks).

### 4. Run

```bash
dottie run
```

This runs your hooks, then creates symlinks (e.g., `~/.zshrc` -> `~/dotfiles/home/zshrc`).

Use `dottie status` to check what's linked.

## Bootstrap a New Machine

`dottie init` generates a `scripts/bootstrap.sh` in your dotfiles repo. Push it to GitHub, then bootstrap any new machine with:

```bash
curl -fsSL https://raw.githubusercontent.com/YOUR_USERNAME/dotfiles/main/scripts/bootstrap.sh | bash
```

This clones your dotfiles, installs dottie, and runs `dottie run`.

## Commands

### `dottie init [dir]`

Initialize a new dotfiles repository structure.

```bash
dottie init              # current directory
```

### `dottie run`

Run hooks and create symlinks from your dotfiles repo to your home directory. Runs pre-link hooks, creates symlinks, then runs post-link hooks.

```bash
dottie run
```

### `dottie hooks list`

List active hooks (executable files in `hooks/`).

```bash
dottie hooks list
```

### `dottie hooks run <phase>`

Run hooks for a specific phase without linking.

```bash
dottie hooks run pre-link   # run pre-link hooks only
dottie hooks run post-link  # run post-link hooks only
dottie hooks run status     # run status hooks only
dottie hooks run pre-link -n  # dry-run
```

### `dottie status`

Show the status of your dotfiles (linked, missing, conflicts), then runs status hooks.

```bash
dottie status
```

## Configuration

`dottie.yaml`:

```yaml
# Where dotfiles are stored in this repo
source_dir: home

# Where to link them (default: $HOME)
target_dir: ~

# Prepend . to filenames when linking
# e.g., home/vimrc -> ~/.vimrc
add_dot: true

# Where to store backups of existing files
backup_dir: ~/.dottie.backup

# What to do on conflict: backup | skip | overwrite
conflict: backup

# Files/directories to ignore
ignore:
  - README.md
  - LICENSE
```

## Hooks

Hooks let you run custom scripts for tasks beyond symlinking dotfiles: installing packages (Homebrew, apt), setting up plugins (vim, zsh), configuring tools, and more.

Hooks are executable scripts in the `hooks/` directory. Each script receives the phase as its first argument (`pre-link`, `post-link`, or `status`). Hooks are executed in parallel (starting in alphabetical order).

**Environment variables available to hooks:**
- `DOTTIE_ROOT` - path to the dotfiles repository
- `DOTTIE_HOME` - target home directory
- `DOTTIE_DRY_RUN` - "true" or "false"

**Phases:**
- `pre-link` - runs before symlinking (install packages here)
- `post-link` - runs after symlinking (source configs, run setup)
- `status` - exit 0 if ok, exit non-zero if needs update

**Example hook:**

```bash
#!/bin/bash
# hooks/01-example.sh

case "$1" in
    pre-link)
        touch ~/.my-setup-complete
        ;;
    post-link)
        echo "Dotfiles linked!"
        ;;
    status)
        if [[ -f ~/.my-setup-complete ]]; then
            exit 0
        else
            exit 1
        fi
        ;;
esac
```

Hidden files (`.foo`) and example files (`*.example.sh`) are skipped.

`dottie init` creates example hooks you can enable by copying:
```bash
cp hooks/homebrew.example.sh hooks/homebrew.sh
```

## Build from Source

```bash
git clone https://github.com/clutchski/dottie.git
cd dottie
make install
```

## License

MIT
