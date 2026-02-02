package init

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_CreatesStructure(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	// Check dottie.yaml exists (not .dottie.yaml)
	assert.FileExists(t, filepath.Join(targetDir, "dottie.yaml"))

	// Check home directory
	assert.DirExists(t, filepath.Join(targetDir, "home"))

	// Check hooks directories
	assert.DirExists(t, filepath.Join(targetDir, "hooks", "pre-install"))
	assert.DirExists(t, filepath.Join(targetDir, "hooks", "post-install"))
	assert.DirExists(t, filepath.Join(targetDir, "hooks", "pre-link"))
	assert.DirExists(t, filepath.Join(targetDir, "hooks", "post-link"))

	// Check deps directory
	assert.DirExists(t, filepath.Join(targetDir, "deps"))

	// Check README
	assert.FileExists(t, filepath.Join(targetDir, "README.md"))
}

func TestInit_CreatesValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	configPath := filepath.Join(targetDir, "dottie.yaml")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	configStr := string(content)
	assert.Contains(t, configStr, "source_dir: home")
	assert.Contains(t, configStr, "target_dir:")
	assert.Contains(t, configStr, "add_dot:")
}

func TestInit_CreatesDepsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(targetDir, "deps", "Brewfile"))
	assert.FileExists(t, filepath.Join(targetDir, "deps", "apt.txt"))
}

func TestInit_CreatesGitkeepFiles(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(targetDir, "hooks", "pre-install", ".gitkeep"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "post-install", ".gitkeep"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "pre-link", ".gitkeep"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "post-link", ".gitkeep"))
}

func TestInit_CreatesExampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	// Check example dotfiles in home/
	assert.FileExists(t, filepath.Join(targetDir, "home", "shellrc"))
	assert.FileExists(t, filepath.Join(targetDir, "home", "editorrc"))
	assert.FileExists(t, filepath.Join(targetDir, "README.md"))

	// Check README has bootstrap instructions
	content, err := os.ReadFile(filepath.Join(targetDir, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "bootstrap.sh")
}

func TestInit_CreatesConfigDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	// Check config directory exists with example file
	assert.DirExists(t, filepath.Join(targetDir, "home", "config"))
	assert.FileExists(t, filepath.Join(targetDir, "home", "config", "starship.toml"))

	// Check content explains directory linking
	content, err := os.ReadFile(filepath.Join(targetDir, "home", "config", "starship.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "~/.config/starship.toml")
}

func TestInit_CreatesBootstrapScript(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, false)
	require.NoError(t, err)

	// Check scripts directory and bootstrap.sh exist
	assert.DirExists(t, filepath.Join(targetDir, "scripts"))
	bootstrapPath := filepath.Join(targetDir, "scripts", "bootstrap.sh")
	assert.FileExists(t, bootstrapPath)

	// Check bootstrap.sh content
	content, err := os.ReadFile(bootstrapPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "#!/bin/bash")
	assert.Contains(t, string(content), "git clone")
	assert.Contains(t, string(content), "dottie run")

	// Check bootstrap.sh is executable
	info, err := os.Stat(bootstrapPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0111 != 0, "bootstrap.sh should be executable")
}

func TestInit_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir, true)
	require.NoError(t, err)

	_, err = os.Stat(targetDir)
	assert.True(t, os.IsNotExist(err), "dry-run should not create directory")
}

func TestInit_FailsIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create dottie.yaml to simulate existing repo
	configPath := filepath.Join(targetDir, "dottie.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0644))

	err := Init(targetDir, false)
	assert.Error(t, err, "should fail if dottie.yaml already exists")
}

func TestInit_WorksWithCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(oldDir)
	}()

	require.NoError(t, os.Chdir(tmpDir))

	err = Init(".", false)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "dottie.yaml"))
}
