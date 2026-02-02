# dottie

A simple dotfiles manager for macOS and Linux.

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/clutchski/dottie/main/scripts/install.sh | bash
```

This auto-detects your OS and architecture and installs the latest release.

Or install a [recent release](https://github.com/clutchski/dottie/releases) manually.

### Build from Source

```bash
git clone https://github.com/clutchski/dottie.git
cd dottie
make install
```

## Quick Start

### Create a New Dotfiles Repo

```bash
mkdir ~/dotfiles && cd ~/dotfiles
git init
dottie init
```

This creates:
- `dottie.yaml` - configuration file
- `home/` - put your dotfiles here (without the leading dot)
- `hooks/` - scripts to run before/after install and link
- `deps/` - dependency files (Brewfile, apt.txt)
- `README.md` - with bootstrap instructions

### Add Your Dotfiles

Copy your dotfiles into `home/` without the leading dot:

```bash
cp ~/.vimrc ~/dotfiles/home/vimrc
cp ~/.zshrc ~/dotfiles/home/zshrc
cp -r ~/.config/nvim ~/dotfiles/home/config/nvim
```

### Link Dotfiles

Preview what will happen:
```bash
dottie link -n  # dry-run
```

Create the symlinks:
```bash
dottie link
```

### Check Status

```bash
dottie status
```

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
dottie init ~/dotfiles   # specific directory
dottie init -n           # dry-run
```

### `dottie link`

Create symlinks from your dotfiles repo to your home directory.

```bash
dottie link              # create symlinks
dottie link -n           # dry-run (show what would happen)
dottie link -f           # force (overwrite existing files)
```

### `dottie install`

Install packages from `deps/Brewfile` (macOS) or `deps/apt.txt` (Linux).

```bash
dottie install           # install packages
dottie install -n        # dry-run
```

### `dottie run <hook>`

Run scripts from a hooks directory.

```bash
dottie run post-install  # run all scripts in hooks/post-install/
dottie run pre-link -n   # dry-run
```

### `dottie status`

Show the status of your dotfiles (linked, missing, conflicts).

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

Place executable scripts in the hooks directories:

- `hooks/pre-install/` - run before `dottie install`
- `hooks/post-install/` - run after `dottie install`
- `hooks/pre-link/` - run before `dottie link`
- `hooks/post-link/` - run after `dottie link`

Scripts are executed in alphabetical order.

## Example Repository Structure

```
dotfiles/
├── dottie.yaml
├── home/
│   ├── vimrc           -> ~/.vimrc
│   ├── zshrc           -> ~/.zshrc
│   ├── tmux.conf       -> ~/.tmux.conf
│   └── config/
│       └── nvim/       -> ~/.config/nvim/
├── hooks/
│   ├── pre-install/
│   ├── post-install/
│   │   └── 01-setup-fzf.sh
│   ├── pre-link/
│   └── post-link/
│       └── 01-source-zshrc.sh
├── deps/
│   ├── Brewfile        # macOS packages
│   └── apt.txt         # Linux packages
└── README.md
```

## License

MIT
