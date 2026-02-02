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
