package status

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clutchski/dottie/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig(t *testing.T, sourceDir, targetDir string) *config.Config {
	t.Helper()

	configContent := `
source_dir: .
add_dot: true
`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0644))

	cfg, err := config.Load(sourceDir)
	require.NoError(t, err)
	cfg.TargetDir = targetDir
	return cfg
}

func TestGetStatus_Linked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create symlink
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(vimrc, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
	assert.Equal(t, "vimrc", statuses[0].Name)
}

func TestGetStatus_NotLinked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file but no target
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusMissing, statuses[0].Status)
}

func TestGetStatus_Diff(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create regular file at target (not a symlink)
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.WriteFile(targetVimrc, []byte("different content"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
}

func TestGetStatus_WrongTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create another file
	otherFile := filepath.Join(tmpDir, "other")
	require.NoError(t, os.WriteFile(otherFile, []byte("other"), 0644))

	// Create symlink pointing to wrong target
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(otherFile, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
}

func TestGetStatus_IgnoresConfiguredFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create files
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	readme := filepath.Join(sourceDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("readme"), 0644))

	// Config that ignores README.md
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

	checker := New(cfg)
	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should only have vimrc, not README.md
	assert.Len(t, statuses, 1)
	assert.Equal(t, "vimrc", statuses[0].Name)
}

func TestFileStatus_String(t *testing.T) {
	assert.Equal(t, "linked", FileStatusLinked.String())
	assert.Equal(t, "missing", FileStatusMissing.String())
	assert.Equal(t, "diff", FileStatusDiff.String())
	assert.Equal(t, "untracked", FileStatusUntracked.String())
}

func TestGetStatus_RecursesIntoExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source config/starship.toml
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))
	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("format = \"$directory\""), 0644))

	// Pre-existing .config directory in target
	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0755))

	// Link starship.toml inside .config
	starshipTarget := filepath.Join(existingConfig, "starship.toml")
	require.NoError(t, os.Symlink(starshipSrc, starshipTarget))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should show config/starship.toml, not just config
	require.Len(t, statuses, 1)
	assert.Equal(t, "config/starship.toml", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestGetStatus_ShowsMultipleFilesInExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source config/ with multiple files
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))

	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("starship config"), 0644))

	alacrittySrc := filepath.Join(sourceConfig, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("alacritty config"), 0644))

	// Pre-existing .config directory with only starship linked
	existingConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(existingConfig, 0755))
	require.NoError(t, os.Symlink(starshipSrc, filepath.Join(existingConfig, "starship.toml")))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should show both files with their individual statuses
	require.Len(t, statuses, 2)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/starship.toml"])
	assert.Equal(t, FileStatusMissing, statusMap["config/alacritty.toml"])
}

func TestGetStatusScan_FindsUntracked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create .bashrc in home (untracked - not in repo)
	bashrc := filepath.Join(targetDir, ".bashrc")
	require.NoError(t, os.WriteFile(bashrc, []byte("# my bashrc"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Find the bashrc status
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].TargetPath == bashrc {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find .bashrc in statuses")
	assert.Equal(t, FileStatusUntracked, found.Status)
}

func TestGetStatusScan_ShowsOk(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create symlink
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(vimrc, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Find the vimrc status
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].TargetPath == targetVimrc {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find .vimrc in statuses")
	assert.Equal(t, FileStatusLinked, found.Status)
}

func TestGetStatusScan_ShowsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file but no symlink in home
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Find the vimrc status (should be missing)
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].TargetPath == targetVimrc {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find .vimrc in statuses")
	assert.Equal(t, FileStatusMissing, found.Status)
}

func TestGetStatusScan_ScansConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create .config/nvim in home (untracked)
	configDir := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	nvimDir := filepath.Join(configDir, "nvim")
	require.NoError(t, os.MkdirAll(nvimDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("-- nvim"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Find the nvim status
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].TargetPath == nvimDir {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find .config/nvim in statuses")
	assert.Equal(t, FileStatusUntracked, found.Status)
}

func TestGetStatusScan_IgnoresNonMatching(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create random dotfile that doesn't match any pattern
	randomFile := filepath.Join(targetDir, ".some_random_file")
	require.NoError(t, os.WriteFile(randomFile, []byte("random"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Should not find the random file
	for _, s := range statuses {
		assert.NotEqual(t, randomFile, s.TargetPath, "random file should not be in statuses")
	}
}

func TestGetStatus_RecursiveDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create deeply nested source: config/foo/bar/baz.toml
	nestedDir := filepath.Join(sourceDir, "config", "foo", "bar")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	bazSrc := filepath.Join(nestedDir, "baz.toml")
	require.NoError(t, os.WriteFile(bazSrc, []byte("nested = true"), 0644))

	// Create pre-existing nested directory structure in target
	targetNested := filepath.Join(targetDir, ".config", "foo", "bar")
	require.NoError(t, os.MkdirAll(targetNested, 0755))

	// Create symlink to the nested file
	bazTarget := filepath.Join(targetNested, "baz.toml")
	require.NoError(t, os.Symlink(bazSrc, bazTarget))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should find the deeply nested file
	require.Len(t, statuses, 1)
	assert.Equal(t, "config/foo/bar/baz.toml", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestGetStatus_RecursiveDirectoriesMixed(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create nested structure: config/nvim/lua/plugins/init.lua
	nestedDir := filepath.Join(sourceDir, "config", "nvim", "lua", "plugins")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	initLua := filepath.Join(nestedDir, "init.lua")
	require.NoError(t, os.WriteFile(initLua, []byte("-- plugins"), 0644))

	// Also create config/nvim/init.lua at a higher level
	nvimDir := filepath.Join(sourceDir, "config", "nvim")
	nvimInit := filepath.Join(nvimDir, "init.lua")
	require.NoError(t, os.WriteFile(nvimInit, []byte("-- nvim init"), 0644))

	// Create pre-existing directory structure in target (some linked, some missing)
	targetNvim := filepath.Join(targetDir, ".config", "nvim")
	targetPlugins := filepath.Join(targetNvim, "lua", "plugins")
	require.NoError(t, os.MkdirAll(targetPlugins, 0755))

	// Link only the top-level init.lua
	require.NoError(t, os.Symlink(nvimInit, filepath.Join(targetNvim, "init.lua")))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should find both files with correct statuses
	require.Len(t, statuses, 2)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/nvim/init.lua"])
	assert.Equal(t, FileStatusMissing, statusMap["config/nvim/lua/plugins/init.lua"])
}

func TestGetStatusScan_ShowsOkForLinkedConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source config/starship.toml
	sourceConfig := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(sourceConfig, 0755))
	starshipSrc := filepath.Join(sourceConfig, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("format = \"test\""), 0644))

	// Create .config directory and symlink starship.toml
	targetConfig := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(targetConfig, 0755))
	starshipTarget := filepath.Join(targetConfig, "starship.toml")
	require.NoError(t, os.Symlink(starshipSrc, starshipTarget))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Find the starship.toml status
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].TargetPath == starshipTarget {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find .config/starship.toml in statuses")
	assert.Equal(t, FileStatusLinked, found.Status, "linked config file should show as ok, not untracked")
}

func TestGetStatusScan_RespectsCustomTargetDir(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "custom-target")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Find the vimrc status
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].TargetPath == targetVimrc {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find .vimrc in statuses")
	assert.Equal(t, FileStatusMissing, found.Status)
	// Target path should NOT start with ~ since target_dir is not HOME
	assert.Equal(t, targetVimrc, found.TargetPath)
	// formatTargetPath should return full path, not ~/
	displayPath := checker.formatTargetPath(found.TargetPath)
	assert.NotContains(t, displayPath, "~", "should not use ~ for custom target_dir")
	assert.Equal(t, targetVimrc, displayPath)
}

// Comprehensive status scenarios

func TestStatus_LinkedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source directory with contents
	vimDir := filepath.Join(sourceDir, "vim")
	require.NoError(t, os.MkdirAll(vimDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(vimDir, "vimrc"), []byte("set number"), 0644))

	// Symlink entire directory
	targetVim := filepath.Join(targetDir, ".vim")
	require.NoError(t, os.Symlink(vimDir, targetVim))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should show the file inside the symlinked directory
	require.Len(t, statuses, 1)
	assert.Equal(t, "vim/vimrc", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestStatus_DiffRegularFileInsteadOfSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create regular file at target (not a symlink)
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.WriteFile(targetVimrc, []byte("different content"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
	assert.Contains(t, statuses[0].Message, "not linked")
}

func TestStatus_DiffSymlinkWrongTarget(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create source file
	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0644))

	// Create another file and symlink to it (wrong target)
	wrongFile := filepath.Join(tmpDir, "wrong")
	require.NoError(t, os.WriteFile(wrongFile, []byte("wrong"), 0644))

	targetVimrc := filepath.Join(targetDir, ".vimrc")
	require.NoError(t, os.Symlink(wrongFile, targetVimrc))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, FileStatusDiff, statuses[0].Status)
	assert.Contains(t, statuses[0].Message, "symlink points to")
}

func TestStatus_DeeplyNestedLinked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create deeply nested source: config/nvim/lua/plugins/lsp.lua
	nestedSrc := filepath.Join(sourceDir, "config", "nvim", "lua", "plugins")
	require.NoError(t, os.MkdirAll(nestedSrc, 0755))
	lspSrc := filepath.Join(nestedSrc, "lsp.lua")
	require.NoError(t, os.WriteFile(lspSrc, []byte("-- lsp config"), 0644))

	// Create matching nested target directory and symlink
	nestedTarget := filepath.Join(targetDir, ".config", "nvim", "lua", "plugins")
	require.NoError(t, os.MkdirAll(nestedTarget, 0755))
	require.NoError(t, os.Symlink(lspSrc, filepath.Join(nestedTarget, "lsp.lua")))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	require.Len(t, statuses, 1)
	assert.Equal(t, "config/nvim/lua/plugins/lsp.lua", statuses[0].Name)
	assert.Equal(t, FileStatusLinked, statuses[0].Status)
}

func TestStatus_MixedStatesInDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create multiple files in config/
	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0755))

	starshipSrc := filepath.Join(configSrc, "starship.toml")
	require.NoError(t, os.WriteFile(starshipSrc, []byte("starship"), 0644))

	alacrittySrc := filepath.Join(configSrc, "alacritty.toml")
	require.NoError(t, os.WriteFile(alacrittySrc, []byte("alacritty"), 0644))

	kittyDir := filepath.Join(configSrc, "kitty")
	require.NoError(t, os.MkdirAll(kittyDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(kittyDir, "kitty.conf"), []byte("kitty"), 0644))

	// Create target directory with mixed states
	configTarget := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(configTarget, 0755))

	// starship: linked correctly
	require.NoError(t, os.Symlink(starshipSrc, filepath.Join(configTarget, "starship.toml")))

	// alacritty: missing (no symlink)

	// kitty: directory exists, file missing
	require.NoError(t, os.MkdirAll(filepath.Join(configTarget, "kitty"), 0755))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/starship.toml"])
	assert.Equal(t, FileStatusMissing, statusMap["config/alacritty.toml"])
	assert.Equal(t, FileStatusMissing, statusMap["config/kitty/kitty.conf"])
}

func TestStatus_ParentSymlinkedShowsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create config directory with multiple files
	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "starship.toml"), []byte("starship"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "alacritty.toml"), []byte("alacritty"), 0644))

	// Symlink entire .config to source config
	require.NoError(t, os.Symlink(configSrc, filepath.Join(targetDir, ".config")))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatus()
	require.NoError(t, err)

	// Should show individual files, not the directory
	require.Len(t, statuses, 2)

	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s.Status
	}

	assert.Equal(t, FileStatusLinked, statusMap["config/starship.toml"])
	assert.Equal(t, FileStatusLinked, statusMap["config/alacritty.toml"])
}

func TestStatusScan_UntrackedInConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create empty source (no dotfiles in repo)

	// Create .config with some files in target (untracked)
	configTarget := filepath.Join(targetDir, ".config")
	require.NoError(t, os.MkdirAll(configTarget, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configTarget, "random.toml"), []byte("random"), 0644))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Should find the untracked file
	var found *DotfileStatus
	for i := range statuses {
		if filepath.Base(statuses[i].TargetPath) == "random.toml" {
			found = &statuses[i]
			break
		}
	}

	require.NotNil(t, found, "expected to find untracked file")
	assert.Equal(t, FileStatusUntracked, found.Status)
}

func TestStatusScan_ShowsFilesWhenParentSymlinked(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "dotfiles")
	targetDir := filepath.Join(tmpDir, "home")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create config in source
	configSrc := filepath.Join(sourceDir, "config")
	require.NoError(t, os.MkdirAll(configSrc, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configSrc, "app.toml"), []byte("app"), 0644))

	// Symlink entire .config directory
	require.NoError(t, os.Symlink(configSrc, filepath.Join(targetDir, ".config")))

	cfg := createTestConfig(t, sourceDir, targetDir)
	checker := New(cfg)

	statuses, err := checker.GetStatusScan()
	require.NoError(t, err)

	// Should show files as linked
	var found *DotfileStatus
	for i := range statuses {
		if statuses[i].Name == "config/app.toml" {
			found = &statuses[i]
			break
		}
	}
	require.NotNil(t, found, "expected to find config/app.toml")
	assert.Equal(t, FileStatusLinked, found.Status)
}
