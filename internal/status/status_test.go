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

func TestGetStatus_Conflict(t *testing.T) {
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
	assert.Equal(t, FileStatusConflict, statuses[0].Status)
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
	assert.Equal(t, FileStatusConflict, statuses[0].Status)
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
	assert.Equal(t, "conflict", FileStatusConflict.String())
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
