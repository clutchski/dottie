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

const exampleShellrc = `# Example: becomes ~/.shellrc when linked
# Replace with your actual dotfiles!

alias ll='ls -la'
`

const exampleEditorrc = `# Example: becomes ~/.editorrc when linked
# Replace with your actual dotfiles!

indent_size = 4
`

const readmeTemplate = `# %s

Bootstrap a new machine:

` + "```" + `bash
curl -fsSL https://raw.githubusercontent.com/clutchski/dottie/main/scripts/bootstrap.sh | bash -s -- --repo %s
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
		fmt.Printf("  %s\n", filepath.Join(absDir, "deps/"))
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

	brewfilePath := filepath.Join(depsPath, "Brewfile")
	if err := os.WriteFile(brewfilePath, []byte(defaultBrewfile), 0644); err != nil {
		return fmt.Errorf("failed to create Brewfile: %w", err)
	}

	aptPath := filepath.Join(depsPath, "apt.txt")
	if err := os.WriteFile(aptPath, []byte(defaultAptTxt), 0644); err != nil {
		return fmt.Errorf("failed to create apt.txt: %w", err)
	}

	// Create README with repo URL
	repoName := filepath.Base(absDir)
	repoURL := getGitRemoteURL(absDir)
	if repoURL == "" {
		repoURL = "https://github.com/YOUR_USERNAME/" + repoName
	}
	readme := fmt.Sprintf(readmeTemplate, repoName, repoURL)
	readmePath := filepath.Join(absDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	return nil
}
