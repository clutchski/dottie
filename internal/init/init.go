package init

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `# Dottie configuration
# See https://github.com/clutchski/dottie for documentation

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
`

const defaultGitignore = `# Backup files
*.backup
*.bak

# OS files
.DS_Store
Thumbs.db

# Editor files
*.swp
*.swo
*~
`

const defaultBrewfile = `# Homebrew packages
# Uncomment or add packages you want to install

# brew "git"
# brew "vim"
# brew "tmux"
# brew "ripgrep"
# brew "fzf"
`

const defaultAptTxt = `# APT packages (one per line)
# Uncomment or add packages you want to install

# git
# vim
# tmux
# ripgrep
# fzf
`

// Init creates a new dotfiles repository structure.
func Init(dir string, dryRun bool) error {
	// Resolve directory
	if dir == "" {
		dir = "."
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	configPath := filepath.Join(absDir, ".dottie.yaml")

	// Check if already initialized
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("directory already contains .dottie.yaml")
	}

	if dryRun {
		fmt.Println("[dry-run] would create:")
		fmt.Printf("  %s\n", configPath)
		fmt.Printf("  %s\n", filepath.Join(absDir, ".gitignore"))
		fmt.Printf("  %s\n", filepath.Join(absDir, "hooks/"))
		fmt.Printf("  %s\n", filepath.Join(absDir, "deps/"))
		return nil
	}

	// Create main directory
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create .dottie.yaml
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create .gitignore
	gitignorePath := filepath.Join(absDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(defaultGitignore), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Create hooks directories
	hookDirs := []string{
		"hooks/pre-install",
		"hooks/post-install",
		"hooks/pre-link",
		"hooks/post-link",
	}
	for _, hookDir := range hookDirs {
		hookPath := filepath.Join(absDir, hookDir)
		if err := os.MkdirAll(hookPath, 0755); err != nil {
			return fmt.Errorf("failed to create hooks directory: %w", err)
		}
		// Create .gitkeep
		gitkeepPath := filepath.Join(hookPath, ".gitkeep")
		if err := os.WriteFile(gitkeepPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	// Create deps directory
	depsPath := filepath.Join(absDir, "deps")
	if err := os.MkdirAll(depsPath, 0755); err != nil {
		return fmt.Errorf("failed to create deps directory: %w", err)
	}

	// Create Brewfile
	brewfilePath := filepath.Join(depsPath, "Brewfile")
	if err := os.WriteFile(brewfilePath, []byte(defaultBrewfile), 0644); err != nil {
		return fmt.Errorf("failed to create Brewfile: %w", err)
	}

	// Create apt.txt
	aptPath := filepath.Join(depsPath, "apt.txt")
	if err := os.WriteFile(aptPath, []byte(defaultAptTxt), 0644); err != nil {
		return fmt.Errorf("failed to create apt.txt: %w", err)
	}

	return nil
}
