package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".dottie.yaml"

// Config represents the dottie configuration.
type Config struct {
	SourceDir string   `yaml:"source_dir"`
	TargetDir string   `yaml:"target_dir"`
	AddDot    bool     `yaml:"add_dot"`
	BackupDir string   `yaml:"backup_dir"`
	Conflict  string   `yaml:"conflict"`
	Ignore    []string `yaml:"ignore"`
	DepsDir   string   `yaml:"deps_dir"`
	HooksDir  string   `yaml:"hooks_dir"`

	// Internal fields
	repoRoot string
}

// alwaysIgnored are paths that are always ignored when linking.
var alwaysIgnored = []string{
	".git",
	".dottie.yaml",
	"hooks",
	"deps",
}

// Load loads the configuration from the given directory.
// If no config file exists, returns a default configuration.
func Load(dir string) (*Config, error) {
	home, _ := os.UserHomeDir()

	cfg := &Config{
		// Set defaults
		SourceDir: ".",
		TargetDir: home,
		AddDot:    true,
		BackupDir: "~/.dottie.backup",
		Conflict:  "backup",
		DepsDir:   "deps",
		HooksDir:  "hooks",
		repoRoot:  dir,
	}

	configPath := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file - use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) > 0 {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Apply defaults for empty values
	if cfg.SourceDir == "" {
		cfg.SourceDir = "."
	}
	if cfg.TargetDir == "" {
		home, _ := os.UserHomeDir()
		cfg.TargetDir = home
	}
	if cfg.BackupDir == "" {
		cfg.BackupDir = "~/.dottie.backup"
	}
	if cfg.Conflict == "" {
		cfg.Conflict = "backup"
	}
	if cfg.DepsDir == "" {
		cfg.DepsDir = "deps"
	}
	if cfg.HooksDir == "" {
		cfg.HooksDir = "hooks"
	}

	cfg.repoRoot = dir

	return cfg, nil
}

// ShouldIgnore returns true if the given path should be ignored.
func (c *Config) ShouldIgnore(path string) bool {
	base := filepath.Base(path)

	// Check always ignored
	for _, ignored := range alwaysIgnored {
		if base == ignored {
			return true
		}
	}

	// Check hooks and deps directories
	if base == c.HooksDir || base == c.DepsDir {
		return true
	}

	// Check user-configured ignores
	for _, pattern := range c.Ignore {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
		// Also check exact match
		if pattern == base {
			return true
		}
	}

	return false
}

// GetSourcePath returns the absolute path to the source directory.
func (c *Config) GetSourcePath() string {
	if filepath.IsAbs(c.SourceDir) {
		return c.SourceDir
	}
	return filepath.Join(c.repoRoot, c.SourceDir)
}

// GetTargetPath returns the target path for a dotfile.
// If AddDot is true, it prepends a dot to the filename.
func (c *Config) GetTargetPath(name string) string {
	targetName := name
	if c.AddDot && !strings.HasPrefix(name, ".") {
		targetName = "." + name
	}
	return filepath.Join(c.TargetDir, targetName)
}

// GetHooksPath returns the absolute path to the hooks directory.
func (c *Config) GetHooksPath() string {
	return filepath.Join(c.repoRoot, c.HooksDir)
}

// GetDepsPath returns the absolute path to the deps directory.
func (c *Config) GetDepsPath() string {
	return filepath.Join(c.repoRoot, c.DepsDir)
}

// GetBackupPath returns the expanded backup directory path.
func (c *Config) GetBackupPath() (string, error) {
	if strings.HasPrefix(c.BackupDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, c.BackupDir[1:]), nil
	}
	return c.BackupDir, nil
}

// RepoRoot returns the root directory of the dotfiles repository.
func (c *Config) RepoRoot() string {
	return c.repoRoot
}
