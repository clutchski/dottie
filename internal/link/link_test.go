package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clutchski/dottie/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig(t *testing.T, sourceDir, targetDir, backupDir string) *config.Config {
	t.Helper()

	configContent := `
source_dir: .
add_dot: true
conflict: backup
`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0644))

	cfg, err := config.Load(sourceDir)
	require.NoError(t, err)
	cfg.TargetDir = targetDir
	cfg.BackupDir = backupDir
	return cfg
}

func TestLink_CreatesSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	expectedTarget := filepath.Join(targetDir, ".vimrc")
	assert.True(t, isSymlink(expectedTarget), "expected symlink at %s", expectedTarget)

	linkTarget, err := os.Readlink(expectedTarget)
	require.NoError(t, err)
	assert.Equal(t, vimrc, linkTarget)

	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)
}

func TestLink_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(true, false) // dry-run = true
	require.NoError(t, err)

	expectedTarget := filepath.Join(targetDir, ".vimrc")
	assert.False(t, fileExists(expectedTarget), "dry-run should not create symlink")

	require.Len(t, results, 1)
	assert.Equal(t, StatusWouldLink, results[0].Status)
}

func TestLink_BackupsExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	existingVimrc := filepath.Join(targetDir, ".vimrc")
	existingContent := "existing content"
	require.NoError(t, os.WriteFile(existingVimrc, []byte(existingContent), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	assert.True(t, isSymlink(existingVimrc), "expected symlink at %s", existingVimrc)

	files, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	require.Len(t, files, 1, "expected 1 backup file")

	backupContent, err := os.ReadFile(filepath.Join(backupDir, files[0].Name()))
	require.NoError(t, err)
	assert.Equal(t, existingContent, string(backupContent))

	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)
	assert.NotEmpty(t, results[0].BackupPath)
}

func TestLink_ForceOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	existingVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.WriteFile(existingVimrc, []byte("existing content"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, true) // force = true
	require.NoError(t, err)

	assert.True(t, isSymlink(existingVimrc), "expected symlink at %s", existingVimrc)

	if fileExists(backupDir) {
		files, _ := os.ReadDir(backupDir)
		assert.Empty(t, files, "expected no backup files with force")
	}

	require.Len(t, results, 1)
	assert.Empty(t, results[0].BackupPath)
}

func TestLink_SkipsAlreadyLinked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	existingVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(vimrc, existingVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusAlreadyLinked, results[0].Status)
}

func TestLink_LinksDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	expectedTarget := filepath.Join(targetDir, ".config")
	assert.True(t, isSymlink(expectedTarget), "expected symlink at %s", expectedTarget)

	linkedInitLua := filepath.Join(expectedTarget, "nvim", "init.lua")
	content, err := os.ReadFile(linkedInitLua)
	require.NoError(t, err)
	assert.Contains(t, string(content), "nvim config")
}

func TestLink_IgnoresConfiguredFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))
	readme := filepath.Join(sourceDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("readme"), 0644))

	configContent := `
source_dir: .
add_dot: true
ignore:
  - README.md
`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0644))

	cfg, err := config.Load(sourceDir)
	require.NoError(t, err)
	cfg.TargetDir = targetDir
	cfg.BackupDir = backupDir

	linker := New(cfg)
	results, err := linker.Link(false, false)
	require.NoError(t, err)

	assert.Len(t, results, 1, "should only link vimrc, not README.md")

	readmeTarget := filepath.Join(targetDir, ".README.md")
	assert.False(t, fileExists(readmeTarget), "README.md should not be linked")
}

func TestLink_LinksIntoExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source config/starship.toml
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))
	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("format = \"$directory\""), 0644))

	// Pre-existing .config/nvim in target (simulating user's existing config)
	existingNvim := filepath.Join(targetDir, ".config", "nvim")
	require.NoError(t, os.MkdirAll(existingNvim, 0755))
	existingInitLua := filepath.Join(existingNvim, "init.lua")
	require.NoError(t, os.WriteFile(existingInitLua, []byte("-- user's nvim config"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// .config should NOT be a symlink (it already existed as a directory)
	targetConfig := filepath.Join(targetDir, ".config")
	assert.False(t, isSymlink(targetConfig), ".config should not be a symlink when it already exists")
	assert.True(t, isDir(targetConfig), ".config should still be a directory")

	// starship.toml should be linked inside .config
	linkedStarship := filepath.Join(targetConfig, "starship.toml")
	assert.True(t, isSymlink(linkedStarship), "starship.toml should be a symlink")
	content, err := os.ReadFile(linkedStarship)
	require.NoError(t, err)
	assert.Contains(t, string(content), "format")

	// nvim should be untouched
	assert.False(t, isSymlink(existingNvim), "existing nvim should not be touched")
	nvimContent, err := os.ReadFile(existingInitLua)
	require.NoError(t, err)
	assert.Equal(t, "-- user's nvim config", string(nvimContent))
}

func TestLink_PreservesExistingFilesInDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source config/alacritty.toml
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))
	alacrittySrc := filepath.Join(sourceConfig, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("font_size = 14"), 0644))

	// Pre-existing .config with nvim and karabiner (not in dotfiles)
	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0755))

	existingNvim := filepath.Join(existingConfig, "nvim")
	require.NoError(t, os.MkdirAll(existingNvim, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(existingNvim, "init.lua"), []byte("-- nvim"), 0644))

	existingKarabiner := filepath.Join(existingConfig, "karabiner")
	require.NoError(t, os.MkdirAll(existingKarabiner, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(existingKarabiner, "config.json"), []byte("{}"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// All three should exist in .config
	assert.True(t, isSymlink(filepath.Join(existingConfig, "alacritty.toml")), "alacritty.toml should be linked")
	assert.True(t, isDir(filepath.Join(existingConfig, "nvim")), "nvim should still exist")
	assert.True(t, isDir(filepath.Join(existingConfig, "karabiner")), "karabiner should still exist")

	// Verify nvim and karabiner are NOT symlinks
	assert.False(t, isSymlink(filepath.Join(existingConfig, "nvim")), "nvim should not be a symlink")
	assert.False(t, isSymlink(filepath.Join(existingConfig, "karabiner")), "karabiner should not be a symlink")
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
