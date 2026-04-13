package init

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_CreatesStructure(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir)
	require.NoError(t, err)

	// Check dottie.yaml exists (not .dottie.yaml)
	assert.FileExists(t, filepath.Join(targetDir, "dottie.yaml"))

	// Check home directory
	assert.DirExists(t, filepath.Join(targetDir, "home"))

	// Check hooks directory (flat, no subdirectories)
	assert.DirExists(t, filepath.Join(targetDir, "hooks"))

	// Check README
	assert.FileExists(t, filepath.Join(targetDir, "README.md"))
}

func TestInit_CreatesValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir)
	require.NoError(t, err)

	configPath := filepath.Join(targetDir, "dottie.yaml")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	configStr := string(content)
	assert.Contains(t, configStr, "source_dir: home")
	assert.Contains(t, configStr, "target_dir:")
	assert.Contains(t, configStr, "add_dot:")
}

func TestInit_CreatesExampleHooks(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir)
	require.NoError(t, err)

	// Check example hook library exists
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "hook.example.sh"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "homebrew.example.sh"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "mise.example.sh"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "apt.example.sh"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "lib.example.sh"))
	assert.FileExists(t, filepath.Join(targetDir, "hooks", "README.md"))

	// Check hook.example.sh content
	hookContent, err := os.ReadFile(filepath.Join(targetDir, "hooks", "hook.example.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(hookContent), "#!/bin/bash")
	assert.Contains(t, string(hookContent), "pre-link")
	assert.Contains(t, string(hookContent), "post-link")
	assert.Contains(t, string(hookContent), "status")

	// Check homebrew.example.sh content
	brewContent, err := os.ReadFile(filepath.Join(targetDir, "hooks", "homebrew.example.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(brewContent), "#!/bin/bash")
	assert.Contains(t, string(brewContent), "DOTTIE_ROOT")
	assert.Contains(t, string(brewContent), "brew bundle")
	assert.Contains(t, string(brewContent), "missing Brewfile")

	miseContent, err := os.ReadFile(filepath.Join(targetDir, "hooks", "mise.example.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(miseContent), "#!/bin/bash")
	assert.Contains(t, string(miseContent), "missing mise.toml")

	aptContent, err := os.ReadFile(filepath.Join(targetDir, "hooks", "apt.example.sh"))
	require.NoError(t, err)
	assert.Contains(t, string(aptContent), "#!/bin/bash")
	assert.Contains(t, string(aptContent), "missing Aptfile")

	// Check example hooks are executable
	hookInfo, err := os.Stat(filepath.Join(targetDir, "hooks", "hook.example.sh"))
	require.NoError(t, err)
	assert.NotEqual(t, fs.FileMode(0), hookInfo.Mode()&0o111, "hook.example.sh should be executable")

	brewInfo, err := os.Stat(filepath.Join(targetDir, "hooks", "homebrew.example.sh"))
	require.NoError(t, err)
	assert.NotEqual(t, fs.FileMode(0), brewInfo.Mode()&0o111, "homebrew.example.sh should be executable")

	miseInfo, err := os.Stat(filepath.Join(targetDir, "hooks", "mise.example.sh"))
	require.NoError(t, err)
	assert.NotEqual(t, fs.FileMode(0), miseInfo.Mode()&0o111, "mise.example.sh should be executable")

	aptInfo, err := os.Stat(filepath.Join(targetDir, "hooks", "apt.example.sh"))
	require.NoError(t, err)
	assert.NotEqual(t, fs.FileMode(0), aptInfo.Mode()&0o111, "apt.example.sh should be executable")
}

func TestInit_CreatesExampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir)
	require.NoError(t, err)

	// Check example dotfiles in home/
	assert.FileExists(t, filepath.Join(targetDir, "home", "shellrc"))
	assert.FileExists(t, filepath.Join(targetDir, "home", "editorrc"))
	assert.FileExists(t, filepath.Join(targetDir, "README.md"))

	// Check README has bootstrap instructions
	content, err := os.ReadFile(filepath.Join(targetDir, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "bootstrap.sh")

	// Check package manifest examples
	assert.FileExists(t, filepath.Join(targetDir, "Brewfile.example"))
	assert.FileExists(t, filepath.Join(targetDir, "Aptfile.example"))
	assert.NoFileExists(t, filepath.Join(targetDir, "mise.toml.example"))

	brewfileContent, err := os.ReadFile(filepath.Join(targetDir, "Brewfile.example"))
	require.NoError(t, err)
	assert.Contains(t, string(brewfileContent), `# tap "clutchski/tap"`)
	assert.Contains(t, string(brewfileContent), `# brew "dottie"`)
	assert.Contains(t, string(brewfileContent), `# cask "iterm2"`)
}

func TestInit_CreatesConfigDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	err := Init(targetDir)
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

	err := Init(targetDir)
	require.NoError(t, err)

	// Check scripts directory and bootstrap.sh exist
	assert.DirExists(t, filepath.Join(targetDir, "scripts"))
	bootstrapPath := filepath.Join(targetDir, "scripts", "bootstrap.sh")
	assert.FileExists(t, bootstrapPath)

	// Check bootstrap.sh content
	content, err := os.ReadFile(bootstrapPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "#!/bin/bash")
	assert.Contains(t, string(content), "DOTFILES_REPO")
	assert.Contains(t, string(content), "clutchski/dottie/main/scripts/bootstrap.sh")

	// Check bootstrap.sh is executable
	info, err := os.Stat(bootstrapPath)
	require.NoError(t, err)
	assert.NotEqual(t, fs.FileMode(0), info.Mode()&0o111, "bootstrap.sh should be executable")
}

func TestInit_FailsIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "dotfiles")

	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create dottie.yaml to simulate existing repo
	configPath := filepath.Join(targetDir, "dottie.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o644))

	err := Init(targetDir)
	assert.Error(t, err, "should fail if dottie.yaml already exists")
}

func TestInit_WorksWithCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Chdir(oldDir))
	}()

	require.NoError(t, os.Chdir(tmpDir))

	err = Init(".")
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "dottie.yaml"))
}
