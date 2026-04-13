package init

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed templates/**
var templateFS embed.FS

func readTemplate(path string) ([]byte, error) {
	b, err := templateFS.ReadFile(filepath.Join("templates", path))
	if err != nil {
		return nil, fmt.Errorf("failed to read template %q: %w", path, err)
	}
	return b, nil
}

func writeTemplate(absDir, outPath, templatePath string, perm os.FileMode) error {
	content, err := readTemplate(templatePath)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(absDir, outPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for %q: %w", outPath, err)
	}
	if err := os.WriteFile(fullPath, content, perm); err != nil {
		return fmt.Errorf("failed to write %q: %w", outPath, err)
	}
	return nil
}

func writeTemplateRendered(absDir, outPath, templatePath string, perm os.FileMode, repl map[string]string) error {
	content, err := readTemplate(templatePath)
	if err != nil {
		return err
	}
	rendered := string(content)
	for k, v := range repl {
		rendered = strings.ReplaceAll(rendered, k, v)
	}
	fullPath := filepath.Join(absDir, outPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for %q: %w", outPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(rendered), perm); err != nil {
		return fmt.Errorf("failed to write %q: %w", outPath, err)
	}
	return nil
}

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
func Init(dir string) error {
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

	// Create main directory
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Core scaffolding
	if err := writeTemplate(absDir, "dottie.yaml", "dottie.yaml", 0o644); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "home/shellrc", "home/shellrc", 0o644); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "home/editorrc", "home/editorrc", 0o644); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "home/config/starship.toml", "home/config/starship.toml", 0o644); err != nil {
		return err
	}

	// Hook library templates
	if err := writeTemplate(absDir, "hooks/hook.example.sh", "hooks/hook.example.sh", 0o755); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "hooks/homebrew.example.sh", "hooks/homebrew.example.sh", 0o755); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "hooks/mise.example.sh", "hooks/mise.example.sh", 0o755); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "hooks/apt.example.sh", "hooks/apt.example.sh", 0o755); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "hooks/lib.example.sh", "hooks/lib.example.sh", 0o644); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "hooks/README.md", "hooks/README.md", 0o644); err != nil {
		return err
	}

	// Optional manifest examples
	if err := writeTemplate(absDir, "Brewfile.example", "Brewfile.example", 0o644); err != nil {
		return err
	}
	if err := writeTemplate(absDir, "Aptfile.example", "Aptfile.example", 0o644); err != nil {
		return err
	}

	repoName := filepath.Base(absDir)
	repoURL := getGitRemoteURL(absDir)
	if repoURL == "" {
		repoURL = "https://github.com/YOUR_USERNAME/" + repoName
	}
	rawURL := strings.Replace(repoURL, "github.com", "raw.githubusercontent.com", 1)

	if err := writeTemplateRendered(absDir, "scripts/bootstrap.sh", "scripts/bootstrap.sh.tmpl", 0o755, map[string]string{
		"__REPO_URL__": repoURL,
	}); err != nil {
		return err
	}

	if err := writeTemplateRendered(absDir, "README.md", "README.md.tmpl", 0o644, map[string]string{
		"__REPO_NAME__": repoName,
		"__RAW_URL__":   rawURL,
	}); err != nil {
		return err
	}

	return nil
}
