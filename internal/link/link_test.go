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
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	existingVimrc := filepath.Join(targetDir, ".vimrc")
	existingContent := "existing content"
	require.NoError(t, os.WriteFile(existingVimrc, []byte(existingContent), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	existingVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.WriteFile(existingVimrc, []byte("existing content"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, true) // force = true
	require.NoError(t, err)

	assert.True(t, isSymlink(existingVimrc), "expected symlink at %s", existingVimrc)

	if fileExists(backupDir) {
		files, err := os.ReadDir(backupDir)
		require.NoError(t, err)
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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))
	readme := filepath.Join(sourceDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("readme"), 0o644))

	configContent := `
source_dir: .
add_dot: true
ignore:
  - README.md
`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create source config/starship.toml
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0o755))
	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("format = \"$directory\""), 0o644))

	// Pre-existing .config/nvim in target (simulating user's existing config)
	existingNvim := filepath.Join(targetDir, ".config", "nvim")
	require.NoError(t, os.MkdirAll(existingNvim, 0o755))
	existingInitLua := filepath.Join(existingNvim, "init.lua")
	require.NoError(t, os.WriteFile(existingInitLua, []byte("-- user's nvim config"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create source config/alacritty.toml
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0o755))
	alacrittySrc := filepath.Join(sourceConfig, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("font_size = 14"), 0o644))

	// Pre-existing .config with nvim and karabiner (not in dotfiles)
	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0o755))

	existingNvim := filepath.Join(existingConfig, "nvim")
	require.NoError(t, os.MkdirAll(existingNvim, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(existingNvim, "init.lua"), []byte("-- nvim"), 0o644))

	existingKarabiner := filepath.Join(existingConfig, "karabiner")
	require.NoError(t, os.MkdirAll(existingKarabiner, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(existingKarabiner, "config.json"), []byte("{}"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create a nested directory structure with files
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0o644))
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create files and directories, some of which should be ignored
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	// Create an ignored directory with files
	gitDir := filepath.Join(sourceDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create source config/nvim/init.lua
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0o644))

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

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	require.NoError(t, os.MkdirAll(otherPlace, 0o755))

	// Create source config/nvim/init.lua
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	initLua := filepath.Join(configDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- nvim config"), 0o644))

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

func TestComputeTargetPath(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

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

// --- Check tests ---

func createCheckConfig(t *testing.T, sourceDir, targetDir string) *config.Config {
	t.Helper()

	configContent := `
source_dir: .
add_dot: true
`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0o644))

	cfg, err := config.Load(sourceDir)
	require.NoError(t, err)
	cfg.TargetDir = targetDir
	return cfg
}

func TestCheck_Linked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))
	require.NoError(t, os.Symlink(vimrc, filepath.Join(targetDir, ".vimrc")))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusLinked, results[0].Status)
	assert.Equal(t, "vimrc", results[0].Name)
}

func TestCheck_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusMissing, results[0].Status)
	assert.Equal(t, "not linked", results[0].Message)
}

func TestCheck_Diff_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".vimrc"), []byte("different"), 0o644))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusDiff, results[0].Status)
	assert.Contains(t, results[0].Message, "not linked")
}

func TestCheck_Diff_WrongSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))

	otherFile := filepath.Join(tmpDir, "other")
	require.NoError(t, os.WriteFile(otherFile, []byte("other"), 0o644))
	require.NoError(t, os.Symlink(otherFile, filepath.Join(targetDir, ".vimrc")))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusDiff, results[0].Status)
	assert.Contains(t, results[0].Message, "symlink points to")
}

func TestCheck_IgnoresConfiguredFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("readme"), 0o644))

	configContent := `
source_dir: .
add_dot: true
ignore:
  - README.md
`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0o644))

	cfg, err := config.Load(sourceDir)
	require.NoError(t, err)
	cfg.TargetDir = targetDir

	linker := New(cfg)
	results, err := linker.Check()
	require.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "vimrc", results[0].Name)
}

func TestCheck_RecursesIntoDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0o755))
	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("format"), 0o644))

	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0o755))
	require.NoError(t, os.Symlink(starshipSrc, filepath.Join(existingConfig, "starship.toml")))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, filepath.Join("config", "starship.toml"), results[0].Name)
	assert.Equal(t, StatusLinked, results[0].Status)
}

func TestCheck_MixedStatesInDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0o755))

	starshipSrc := filepath.Join(configSrc, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("starship"), 0o644))

	alacrittySrc := filepath.Join(configSrc, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("alacritty"), 0o644))

	configTarget := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(configTarget, 0o755))
	require.NoError(t, os.Symlink(starshipSrc, filepath.Join(configTarget, "starship.toml")))
	// alacritty not linked

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 2)

	statusMap := make(map[string]Status)
	for _, r := range results {
		statusMap[r.Name] = r.Status
	}

	assert.Equal(t, StatusLinked, statusMap[filepath.Join("config", "starship.toml")])
	assert.Equal(t, StatusMissing, statusMap[filepath.Join("config", "alacritty.toml")])
}

func TestCheck_ParentSymlinkedShowsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "starship.toml"), []byte("starship"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "alacritty.toml"), []byte("alacritty"), 0o644))

	// Symlink entire .config to source config
	require.NoError(t, os.Symlink(configSrc, filepath.Join(targetDir, ".config")))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 2)

	statusMap := make(map[string]Status)
	for _, r := range results {
		statusMap[r.Name] = r.Status
	}

	assert.Equal(t, StatusLinked, statusMap[filepath.Join("config", "starship.toml")])
	assert.Equal(t, StatusLinked, statusMap[filepath.Join("config", "alacritty.toml")])
}

func TestCheck_SetsNameAndTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createCheckConfig(t, sourceDir, targetDir)
	linker := New(cfg)

	results, err := linker.Check()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, "vimrc", results[0].Name)
	assert.Equal(t, vimrc, results[0].Source)
	assert.Equal(t, filepath.Join(targetDir, ".vimrc"), results[0].Target)
}

// --- Prune tests ---

func TestCheck_IncludesDanglingFromManifest(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create two source files and link them (creates manifest)
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))
	bashrc := filepath.Join(sourceDir, "bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("export PATH"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Delete bashrc source to make its symlink dangle
	require.NoError(t, os.Remove(bashrc))

	results, err := linker.Check()
	require.NoError(t, err)

	// Should include the live vimrc AND the dangling bashrc
	statusMap := make(map[Status][]Result)
	for _, r := range results {
		statusMap[r.Status] = append(statusMap[r.Status], r)
	}

	require.Len(t, statusMap[StatusLinked], 1, "vimrc should be linked")
	require.Len(t, statusMap[StatusDangling], 1, "bashrc should be dangling")
	assert.Equal(t, filepath.Join(targetDir, ".bashrc"), statusMap[StatusDangling][0].Target)
}

func TestPrune_NoManifest(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.FindDangling()
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestPrune_FindsDanglingSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create source file, link it, then delete the source
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Delete the source file to make the symlink dangle
	require.NoError(t, os.Remove(vimrc))

	results, err := linker.FindDangling()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusDangling, results[0].Status)
	assert.Equal(t, filepath.Join(targetDir, ".vimrc"), results[0].Target)
}

func TestPrune_NoDanglingSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	results, err := linker.FindDangling()
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestPrune_MixedValidAndDangling(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))
	bashrc := filepath.Join(sourceDir, "bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("export PATH"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Delete only bashrc source to make that symlink dangle
	require.NoError(t, os.Remove(bashrc))

	results, err := linker.FindDangling()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusDangling, results[0].Status)
	assert.Equal(t, filepath.Join(targetDir, ".bashrc"), results[0].Target)
}

func TestPrune_NestedDanglingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create nested source file
	configDir := filepath.Join(sourceDir, "config", "nvim")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	initVim := filepath.Join(configDir, "init.vim")
	require.NoError(t, os.WriteFile(initVim, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Delete source to make symlink dangle
	require.NoError(t, os.Remove(initVim))

	results, err := linker.FindDangling()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, StatusDangling, results[0].Status)
	expectedTarget := filepath.Join(targetDir, ".config", "nvim", "init.vim")
	assert.Equal(t, expectedTarget, results[0].Target)
}

func TestPrune_RemovedSymlinkIsCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Remove the symlink itself (user deleted it manually)
	require.NoError(t, os.Remove(filepath.Join(targetDir, ".vimrc")))

	results, err := linker.FindDangling()
	require.NoError(t, err)

	// The symlink is gone so it should be reported for manifest cleanup
	require.Len(t, results, 1)
	assert.Equal(t, StatusDangling, results[0].Status)
}

func TestLink_RecordsManifest(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))
	bashrc := filepath.Join(sourceDir, "bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("export PATH"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	manifestPath := filepath.Join(targetDir, ".dottie.links")
	entries, err := loadManifest(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, []string{
		filepath.Join(targetDir, ".bashrc"),
		filepath.Join(targetDir, ".vimrc"),
	}, entries)
}

func TestLink_DryRunDoesNotWriteManifest(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(true, false) // dry-run
	require.NoError(t, err)

	manifestPath := filepath.Join(targetDir, ".dottie.links")
	assert.False(t, fileExists(manifestPath), "dry-run should not create manifest")
}

func TestLink_SetsName(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	results, err := linker.Link(false, false)
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.Equal(t, "vimrc", results[0].Name)
}

func TestPrune_ReportsRemovalError(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Delete source to make symlink dangle
	require.NoError(t, os.Remove(vimrc))

	// Make the target's parent directory read-only to prevent removal
	require.NoError(t, os.Chmod(targetDir, 0o555))
	t.Cleanup(func() {
		if err := os.Chmod(targetDir, 0o755); err != nil {
			t.Logf("cleanup chmod: %v", err)
		}
	})

	results, err := linker.Prune()
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, StatusDangling, results[0].Status)
	require.Error(t, results[0].Error, "should report removal error")

	// Manifest should still contain the entry since removal failed
	require.NoError(t, os.Chmod(targetDir, 0o755))
	entries, err := loadManifest(filepath.Join(targetDir, ".dottie.links"))
	require.NoError(t, err)
	assert.Contains(t, entries, filepath.Join(targetDir, ".vimrc"))
}

func TestPrune_RemovesDanglingAndUpdatesManifest(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backup")

	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))
	bashrc := filepath.Join(sourceDir, "bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("export PATH"), 0o644))

	cfg := createTestConfig(t, sourceDir, targetDir, backupDir)
	linker := New(cfg)

	_, err := linker.Link(false, false)
	require.NoError(t, err)

	// Delete bashrc source to make its symlink dangle
	require.NoError(t, os.Remove(bashrc))

	results, err := linker.Prune()
	require.NoError(t, err)

	// Should report the dangling link
	require.Len(t, results, 1)
	assert.Equal(t, StatusDangling, results[0].Status)
	assert.Equal(t, filepath.Join(targetDir, ".bashrc"), results[0].Target)

	// Dangling symlink should be removed
	assert.False(t, fileExists(filepath.Join(targetDir, ".bashrc")))

	// Manifest should only contain the valid link
	manifestPath := filepath.Join(targetDir, ".dottie.links")
	entries, err := loadManifest(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(targetDir, ".vimrc")}, entries)
}
