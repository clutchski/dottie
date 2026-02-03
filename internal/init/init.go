package init

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultConfig = `# Dottie configuration

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
`

const exampleShellrc = `# Example: becomes ~/.shellrc when linked
# Replace with your actual dotfiles!

alias ll='ls -la'
`

const exampleEditorrc = `# Example: becomes ~/.editorrc when linked
# Replace with your actual dotfiles!

indent_size = 4
`

const exampleStarship = `# Example: becomes ~/.config/starship.toml when linked
# Shows how to link files into directories that already exist

format = "$directory$git_branch$character"
`

const hookExampleTemplate = `#!/bin/bash
# Example hook template
# Copy this file: cp hooks/hook.example.sh hooks/my-hook.sh
# Then edit my-hook.sh to add your logic

case "$1" in
    pre-link)
        echo "Running pre-link for my-hook"
        ;;
    post-link)
        echo "Running post-link for my-hook"
        ;;
    status)
        echo "my-hook status: ok"
        ;;
esac
`

const homebrewExampleTemplate = `#!/bin/bash
# Homebrew package management hook
# To enable: cp hooks/homebrew.example.sh hooks/homebrew.sh
set -e
BREWFILE="$DOTTIE_ROOT/Brewfile"

case "$1" in
    pre-link)
        if [[ -f "$BREWFILE" ]]; then
            if [[ "$DOTTIE_DRY_RUN" == "true" ]]; then
                echo "[dry-run] would run: brew bundle --file=$BREWFILE"
            else
                brew bundle check --file="$BREWFILE" 2>/dev/null || brew bundle --file="$BREWFILE"
            fi
        fi
        ;;
    status)
        if [[ -f "$BREWFILE" ]]; then
            echo "Brewfile:"
            if brew bundle check --file="$BREWFILE" 2>/dev/null; then
                echo "  All packages installed"
            else
                brew bundle check --file="$BREWFILE" --verbose 2>&1 | grep "needs to be installed" | sed 's/^/  /' || true
            fi
        fi
        ;;
esac
`

const bootstrapScript = `#!/bin/bash
set -e

REPO="%s"

git clone "$REPO" ~/.dotfiles
cd ~/.dotfiles

curl -fsSL https://raw.githubusercontent.com/clutchski/dottie/main/scripts/install.sh | bash

dottie link
`

const readmeTemplate = `# %s

` + "```" + `bash
curl -fsSL %s/main/scripts/bootstrap.sh | bash
` + "```" + `
`

// getGitRemoteURL tries to get the git remote URL for the current directory.
func getGitRemoteURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))

	// Convert SSH URLs to HTTPS
	// git@github.com:user/repo.git -> https://github.com/user/repo
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.TrimPrefix(url, "git@github.com:")
		url = strings.TrimSuffix(url, ".git")
		url = "https://github.com/" + url
	} else if strings.HasPrefix(url, "https://") {
		url = strings.TrimSuffix(url, ".git")
	}

	return url
}

// Init creates a new dotfiles repository structure.
func Init(dir string, dryRun bool) error {
	if dir == "" {
		dir = "."
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	configPath := filepath.Join(absDir, "dottie.yaml")

	// Check if already initialized
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("directory already contains dottie.yaml")
	}

	if dryRun {
		fmt.Println("[dry-run] would create:")
		fmt.Printf("  %s\n", configPath)
		fmt.Printf("  %s\n", filepath.Join(absDir, "home/"))
		fmt.Printf("  %s\n", filepath.Join(absDir, "hooks/"))
		fmt.Printf("  %s\n", filepath.Join(absDir, "README.md"))
		return nil
	}

	// Create main directory
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create dottie.yaml
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create home directory with examples
	homePath := filepath.Join(absDir, "home")
	if err := os.MkdirAll(homePath, 0755); err != nil {
		return fmt.Errorf("failed to create home directory: %w", err)
	}

	shellrcPath := filepath.Join(homePath, "shellrc")
	if err := os.WriteFile(shellrcPath, []byte(exampleShellrc), 0644); err != nil {
		return fmt.Errorf("failed to create example shellrc: %w", err)
	}

	editorrcPath := filepath.Join(homePath, "editorrc")
	if err := os.WriteFile(editorrcPath, []byte(exampleEditorrc), 0644); err != nil {
		return fmt.Errorf("failed to create example editorrc: %w", err)
	}

	// Create config directory with example (demonstrates linking into existing directories)
	homeConfigPath := filepath.Join(homePath, "config")
	if err := os.MkdirAll(homeConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	starshipPath := filepath.Join(homeConfigPath, "starship.toml")
	if err := os.WriteFile(starshipPath, []byte(exampleStarship), 0644); err != nil {
		return fmt.Errorf("failed to create example starship.toml: %w", err)
	}

	// Create hooks directory with example hooks
	hooksPath := filepath.Join(absDir, "hooks")
	if err := os.MkdirAll(hooksPath, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookExamplePath := filepath.Join(hooksPath, "hook.example.sh")
	if err := os.WriteFile(hookExamplePath, []byte(hookExampleTemplate), 0755); err != nil {
		return fmt.Errorf("failed to create hook.example.sh: %w", err)
	}

	homebrewExamplePath := filepath.Join(hooksPath, "homebrew.example.sh")
	if err := os.WriteFile(homebrewExamplePath, []byte(homebrewExampleTemplate), 0755); err != nil {
		return fmt.Errorf("failed to create homebrew.example.sh: %w", err)
	}

	// Create scripts directory with bootstrap.sh
	scriptsPath := filepath.Join(absDir, "scripts")
	if err := os.MkdirAll(scriptsPath, 0755); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	repoName := filepath.Base(absDir)
	repoURL := getGitRemoteURL(absDir)
	if repoURL == "" {
		repoURL = "https://github.com/YOUR_USERNAME/" + repoName
	}

	bootstrap := fmt.Sprintf(bootstrapScript, repoURL)
	bootstrapPath := filepath.Join(scriptsPath, "bootstrap.sh")
	if err := os.WriteFile(bootstrapPath, []byte(bootstrap), 0755); err != nil {
		return fmt.Errorf("failed to create bootstrap.sh: %w", err)
	}

	// Create README with raw URL for bootstrap
	rawURL := strings.Replace(repoURL, "github.com", "raw.githubusercontent.com", 1)
	readme := fmt.Sprintf(readmeTemplate, repoName, rawURL)
	readmePath := filepath.Join(absDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	return nil
}
