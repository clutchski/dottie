package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".dottie.links")
	got, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestLoadManifest_ReadsLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dottie.links")
	require.NoError(t, os.WriteFile(path, []byte("/home/user/.vimrc\n/home/user/.bashrc\n"), 0o644))

	got, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"/home/user/.bashrc", "/home/user/.vimrc"}, got)
}

func TestLoadManifest_DeduplicatesAndSorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dottie.links")
	require.NoError(t, os.WriteFile(path, []byte("/c\n/a\n/b\n/a\n/c\n"), 0o644))

	got, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"/a", "/b", "/c"}, got)
}

func TestSaveManifest_WritesLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dottie.links")

	require.NoError(t, SaveManifest(path, []string{"/home/user/.vimrc", "/home/user/.bashrc"}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "/home/user/.vimrc\n/home/user/.bashrc\n", string(data))
}

func TestSaveManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dottie.links")

	input := []string{"/a", "/b", "/c"}
	require.NoError(t, SaveManifest(path, input))

	got, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, input, got)
}
