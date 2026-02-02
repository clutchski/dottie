package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBrewfile(t *testing.T) {
	tmpDir := t.TempDir()
	brewfile := filepath.Join(tmpDir, "Brewfile")

	content := `# Comment
brew "git"
brew "vim"
# Another comment
brew "tmux"

cask "visual-studio-code"
tap "homebrew/cask-fonts"
`
	require.NoError(t, os.WriteFile(brewfile, []byte(content), 0644))

	packages, err := ParseBrewfile(brewfile)
	require.NoError(t, err)

	assert.Len(t, packages, 3)
	assert.Contains(t, packages, "git")
	assert.Contains(t, packages, "vim")
	assert.Contains(t, packages, "tmux")
}

func TestParseBrewfile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	brewfile := filepath.Join(tmpDir, "Brewfile")

	require.NoError(t, os.WriteFile(brewfile, []byte("# Just comments\n"), 0644))

	packages, err := ParseBrewfile(brewfile)
	require.NoError(t, err)
	assert.Empty(t, packages)
}

func TestParseBrewfile_MissingFile(t *testing.T) {
	packages, err := ParseBrewfile("/nonexistent/Brewfile")
	assert.NoError(t, err) // Should not error, just return empty
	assert.Empty(t, packages)
}

func TestParseAptFile(t *testing.T) {
	tmpDir := t.TempDir()
	aptFile := filepath.Join(tmpDir, "apt.txt")

	content := `# Comment
git
vim
# Another comment
tmux
`
	require.NoError(t, os.WriteFile(aptFile, []byte(content), 0644))

	packages, err := ParseAptFile(aptFile)
	require.NoError(t, err)

	assert.Len(t, packages, 3)
	assert.Contains(t, packages, "git")
	assert.Contains(t, packages, "vim")
	assert.Contains(t, packages, "tmux")
}

func TestParseAptFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	aptFile := filepath.Join(tmpDir, "apt.txt")

	require.NoError(t, os.WriteFile(aptFile, []byte("# Just comments\n"), 0644))

	packages, err := ParseAptFile(aptFile)
	require.NoError(t, err)
	assert.Empty(t, packages)
}

func TestParseAptFile_MissingFile(t *testing.T) {
	packages, err := ParseAptFile("/nonexistent/apt.txt")
	assert.NoError(t, err) // Should not error, just return empty
	assert.Empty(t, packages)
}

func TestInstaller_GetDepsFile_Darwin(t *testing.T) {
	tmpDir := t.TempDir()
	depsDir := filepath.Join(tmpDir, "deps")
	require.NoError(t, os.MkdirAll(depsDir, 0755))

	brewfile := filepath.Join(depsDir, "Brewfile")
	require.NoError(t, os.WriteFile(brewfile, []byte("brew \"git\""), 0644))

	installer := New(depsDir)

	// Test darwin
	path := installer.GetDepsFile("darwin")
	assert.Equal(t, brewfile, path)
}

func TestInstaller_GetDepsFile_Linux(t *testing.T) {
	tmpDir := t.TempDir()
	depsDir := filepath.Join(tmpDir, "deps")
	require.NoError(t, os.MkdirAll(depsDir, 0755))

	aptFile := filepath.Join(depsDir, "apt.txt")
	require.NoError(t, os.WriteFile(aptFile, []byte("git"), 0644))

	installer := New(depsDir)

	// Test linux
	path := installer.GetDepsFile("linux")
	assert.Equal(t, aptFile, path)
}

func TestInstaller_ListPackages_Darwin(t *testing.T) {
	tmpDir := t.TempDir()
	depsDir := filepath.Join(tmpDir, "deps")
	require.NoError(t, os.MkdirAll(depsDir, 0755))

	brewfile := filepath.Join(depsDir, "Brewfile")
	content := `brew "git"
brew "vim"
`
	require.NoError(t, os.WriteFile(brewfile, []byte(content), 0644))

	installer := New(depsDir)
	packages, err := installer.ListPackages("darwin")
	require.NoError(t, err)

	assert.Len(t, packages, 2)
	assert.Contains(t, packages, "git")
	assert.Contains(t, packages, "vim")
}

func TestInstaller_ListPackages_Linux(t *testing.T) {
	tmpDir := t.TempDir()
	depsDir := filepath.Join(tmpDir, "deps")
	require.NoError(t, os.MkdirAll(depsDir, 0755))

	aptFile := filepath.Join(depsDir, "apt.txt")
	content := `git
vim
`
	require.NoError(t, os.WriteFile(aptFile, []byte(content), 0644))

	installer := New(depsDir)
	packages, err := installer.ListPackages("linux")
	require.NoError(t, err)

	assert.Len(t, packages, 2)
	assert.Contains(t, packages, "git")
	assert.Contains(t, packages, "vim")
}
