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

func TestLink_ForceOverwritesBrokenSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create a broken symlink at target (points to non-existent file)
	existingVimrc := filepath.Join(targetDir, ".vimrc")
	nonExistent := filepath.Join(tmpDir, "does-not-exist")
	require.NoError(t, os.Symlink(nonExistent, existingVimrc))

	// Verify it's a broken symlink
	_, err := os.Stat(existingVimrc)
	require.Error(t, err, "symlink should be broken")
	_, err = os.Lstat(existingVimrc)
	require.NoError(t, err, "symlink itself should exist")

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, true) // force = true
	require.NoError(t, err)

	// Verify the broken symlink was replaced with correct one
	assert.True(t, isSymlink(existingVimrc), "expected symlink at %s", existingVimrc)
	target, err := os.Readlink(existingVimrc)
	require.NoError(t, err)
	assert.Equal(t, vimrc, target, "symlink should point to source file")

	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)
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

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	// New behavior: directories are created as real dirs, only files are symlinked
	targetConfig := filepath.Join(targetDir, ".config")
	assert.True(t, isDir(targetConfig), ".config should be a directory")
	assert.False(t, isSymlink(targetConfig), ".config should NOT be a symlink")

	targetNvim := filepath.Join(targetConfig, "nvim")
	assert.True(t, isDir(targetNvim), "nvim should be a directory")
	assert.False(t, isSymlink(targetNvim), "nvim should NOT be a symlink")

	// The file itself should be symlinked
	linkedInitLua := filepath.Join(targetNvim, "init.lua")
	assert.True(t, isSymlink(linkedInitLua), "init.lua should be a symlink")
	content, err := os.ReadFile(linkedInitLua)
	require.NoError(t, err)
	assert.Contains(t, string(content), "nvim config")

	// Should have exactly one result (for the file)
	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)
	assert.Equal(t, initLua, results[0].Source)
	assert.Equal(t, linkedInitLua, results[0].Target)
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

func TestCollectSourcePaths_CollectsFilesOnly(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create a nested directory structure with files
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0644))
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	sources, err := linker.collectSourcePaths()
	require.NoError(t, err)

	// Should collect only files, not directories
	require.Len(t, sources, 2)
	assert.Contains(t, sources, vimrc)
	assert.Contains(t, sources, initLua)
}

func TestCollectSourcePaths_IgnoresConfiguredDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create files and directories, some of which should be ignored
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create an ignored directory with files
	gitDir := filepath.Join(sourceDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	sources, err := linker.collectSourcePaths()
	require.NoError(t, err)

	// Should only collect vimrc, not files inside .git
	require.Len(t, sources, 1)
	assert.Equal(t, vimrc, sources[0])
}

func TestLink_MigratesOldDirectorySymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source config/nvim/init.lua
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0644))

	// Create old-style directory symlink (simulating previous dottie behavior)
	targetConfig := filepath.Join(targetDir, ".config")
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.Symlink(sourceConfig, targetConfig))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	// Old directory symlink should be replaced with real directory
	assert.False(t, isSymlink(targetConfig), ".config should NOT be a symlink anymore")
	assert.True(t, isDir(targetConfig), ".config should be a real directory")

	// File should be properly symlinked
	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)

	linkedFile := filepath.Join(targetConfig, "nvim", "init.lua")
	assert.True(t, isSymlink(linkedFile), "init.lua should be a symlink")

	linkTarget, err := os.Readlink(linkedFile)
	require.NoError(t, err)
	assert.Equal(t, initLua, linkTarget)
}

func TestLink_ReplacesParentSymlinkWithDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")
	otherPlace := filepath.Join(tmpDir, "other")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.MkdirAll(otherPlace, 0755))

	// Create source config/nvim/init.lua
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0644))

	// Create ~/.config as symlink to a different place (not dotfiles)
	targetConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.Symlink(otherPlace, targetConfig))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	// The parent symlink should have been replaced with a real directory
	assert.False(t, isSymlink(targetConfig), ".config should NOT be a symlink anymore")
	assert.True(t, isDir(targetConfig), ".config should be a real directory")

	// The file should be symlinked
	linkedFile := filepath.Join(targetConfig, "nvim", "init.lua")
	assert.True(t, isSymlink(linkedFile), "init.lua should be a symlink")

	linkTarget, err := os.Readlink(linkedFile)
	require.NoError(t, err)
	assert.Equal(t, initLua, linkTarget)

	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)
}

func TestRun_StreamsEvents(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))
	bashrc := filepath.Join(sourceDir, "bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("export PATH"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	ch, err := linker.Run(false, false)
	require.NoError(t, err)

	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}

	assert.Len(t, events, 2)
	for _, ev := range events {
		assert.Equal(t, StatusLinked, ev.Status)
	}
}

func TestCheckStatus_Linked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(vimrc, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
	assert.Equal(t, "vimrc", statuses[0].Name)
}

func TestCheckStatus_NotLinked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusMissing, statuses[0].Status)
}

func TestCheckStatus_Diff(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.WriteFile(targetVimrc, []byte("different content"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
}

func TestCheckStatus_WrongTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	otherFile := filepath.Join(tmpDir, "other")
	require.NoError(t, os.WriteFile(otherFile, []byte("other"), 0644))

	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(otherFile, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
}

func TestCheckStatus_IgnoresConfiguredFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

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

	linker := New(cfg)
	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	assert.Len(t, statuses, 1)
	assert.Equal(t, "vimrc", statuses[0].Name)
}

func TestFileStatus_String(t *testing.T) {
	assert.Equal(t, "linked", FileStatusLinked.String())
	assert.Equal(t, "unlinked", FileStatusMissing.String())
	assert.Equal(t, "diff", FileStatusDiff.String())
}

func TestCheckStatus_RecursesIntoExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))
	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("format = \"$directory\""), 0644))

	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0755))

	starshipTarget := filepath.Join(existingConfig, "starship.toml")
	require.NoError(t, os.Symlink(starshipSrc, starshipTarget))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, "config/starship.toml", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestCheckStatus_ShowsMultipleFilesInExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))

	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("starship config"), 0644))

	alacrittySrc := filepath.Join(sourceConfig, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("alacritty config"), 0644))

	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0755))
	require.NoError(t, os.Symlink(starshipSrc, filepath.Join(existingConfig, "starship.toml")))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 2)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/starship.toml"])
	assert.Equal(t, FileStatusMissing, statusMap["config/alacritty.toml"])
}

func TestCheckStatus_RecursiveDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	nestedDir := filepath.Join(sourceDir, "config", "foo", "bar")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	bazSrc := filepath.Join(nestedDir, "baz.toml")
	require.NoError(t, os.WriteFile(bazSrc, []byte("nested = true"), 0644))

	targetNested := filepath.Join(targetDir, ".config", "foo", "bar")
	require.NoError(t, os.MkdirAll(targetNested, 0755))

	bazTarget := filepath.Join(targetNested, "baz.toml")
	require.NoError(t, os.Symlink(bazSrc, bazTarget))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, "config/foo/bar/baz.toml", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestCheckStatus_RecursiveDirectoriesMixed(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	nestedDir := filepath.Join(sourceDir, "config", "nvim", "lua", "plugins")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	initLua := filepath.Join(nestedDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- plugins"), 0644))

	nvimDir := filepath.Join(sourceDir, "config", "nvim")
	nvimInit := filepath.Join(nvimDir, "init.lua")
	require.NoError(t, os.WriteFile(nvimInit, []byte("-- nvim init"), 0644))

	targetNvim := filepath.Join(targetDir, ".config", "nvim")
	targetPlugins := filepath.Join(targetNvim, "lua", "plugins")
	require.NoError(t, os.MkdirAll(targetPlugins, 0755))

	require.NoError(t, os.Symlink(nvimInit, filepath.Join(targetNvim, "init.lua")))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 2)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/nvim/init.lua"])
	assert.Equal(t, FileStatusMissing, statusMap["config/nvim/lua/plugins/init.lua"])
}

func TestCheckStatus_LinkedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimDir := filepath.Join(sourceDir, "vim")
	require.NoError(t, os.MkdirAll(vimDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(vimDir, "vimrc"), []byte("set number"), 0644))

	targetVim := filepath.Join(targetDir, ".vim")
	require.NoError(t, os.Symlink(vimDir, targetVim))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, "vim/vimrc", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestCheckStatus_DiffRegularFileInsteadOfSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.WriteFile(targetVimrc, []byte("different content"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
	assert.Contains(t, statuses[0].Message, "not linked")
}

func TestCheckStatus_DiffSymlinkWrongTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	wrongFile := filepath.Join(tmpDir, "wrong")
	require.NoError(t, os.WriteFile(wrongFile, []byte("wrong"), 0644))

	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(wrongFile, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
	assert.Contains(t, statuses[0].Message, "symlink points to")
}

func TestCheckStatus_DeeplyNestedLinked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	nestedSrc := filepath.Join(sourceDir, "config", "nvim", "lua", "plugins")
	require.NoError(t, os.MkdirAll(nestedSrc, 0755))
	lspSrc := filepath.Join(nestedSrc, "lsp.lua")
	require.NoError(t, os.WriteFile(lspSrc, []byte("-- lsp config"), 0644))

	nestedTarget := filepath.Join(targetDir, ".config", "nvim", "lua", "plugins")
	require.NoError(t, os.MkdirAll(nestedTarget, 0755))
	require.NoError(t, os.Symlink(lspSrc, filepath.Join(nestedTarget, "lsp.lua")))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, "config/nvim/lua/plugins/lsp.lua", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestCheckStatus_MixedStatesInDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0755))

	starshipSrc := filepath.Join(configSrc, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("starship"), 0644))

	alacrittySrc := filepath.Join(configSrc, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("alacritty"), 0644))

	kittyDir := filepath.Join(configSrc, "kitty")
	require.NoError(t, os.MkdirAll(kittyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(kittyDir, "kitty.conf"), []byte("kitty"), 0644))

	configTarget := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(configTarget, 0755))

	require.NoError(t, os.Symlink(starshipSrc, filepath.Join(configTarget, "starship.toml")))

	require.NoError(t, os.MkdirAll(filepath.Join(configTarget, "kitty"), 0755))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/starship.toml"])
	assert.Equal(t, FileStatusMissing, statusMap["config/alacritty.toml"])
	assert.Equal(t, FileStatusMissing, statusMap["config/kitty/kitty.conf"])
}

func TestCheckStatus_ParentSymlinkedShowsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "starship.toml"), []byte("starship"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "alacritty.toml"), []byte("alacritty"), 0644))

	require.NoError(t, os.Symlink(configSrc, filepath.Join(targetDir, ".config")))

	cfg := createTestConfig(t, sourceDir, targetDir, "")
	linker := New(cfg)

	statuses, err := linker.CheckStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 2)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/starship.toml"])
	assert.Equal(t, FileStatusLinked, statusMap["config/alacritty.toml"])
}

func TestComputeTargetPath(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	tests := []struct {
		relPath  string
		expected string
	}{
		{"vimrc", filepath.Join(targetDir, ".vimrc")},
		{"config/nvim/init.lua", filepath.Join(targetDir, ".config", "nvim", "init.lua")},
		{"config/starship.toml", filepath.Join(targetDir, ".config", "starship.toml")},
	}

	for _, tc := range tests {
		t.Run(tc.relPath, func(t *testing.T) {
			result := linker.computeTargetPath(tc.relPath)
			assert.Equal(t, tc.expected, result)
		})
	}
}
