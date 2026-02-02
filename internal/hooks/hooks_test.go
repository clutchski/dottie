package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_ExecutesScriptsInOrder(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks", "post-install")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create scripts that append to a file
	script1 := filepath.Join(hooksDir, "01-first.sh")
	require.NoError(t, os.WriteFile(script1, []byte("#!/bin/bash\necho first >> "+outputFile), 0755))

	script2 := filepath.Join(hooksDir, "02-second.sh")
	require.NoError(t, os.WriteFile(script2, []byte("#!/bin/bash\necho second >> "+outputFile), 0755))

	runner := New(filepath.Join(tmpDir, "hooks"))
	err := runner.Run(PostInstall, false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "first\nsecond\n", string(content))
}

func TestRun_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks", "pre-link")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "01-test.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/bash\necho test >> "+outputFile), 0755))

	runner := New(filepath.Join(tmpDir, "hooks"))
	err := runner.Run(PreLink, true) // dry-run
	require.NoError(t, err)

	// Output file should not exist
	assert.False(t, fileExists(outputFile), "dry-run should not execute scripts")
}

func TestRun_SkipsEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks", "pre-install")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create only .gitkeep
	gitkeep := filepath.Join(hooksDir, ".gitkeep")
	require.NoError(t, os.WriteFile(gitkeep, []byte(""), 0644))

	runner := New(filepath.Join(tmpDir, "hooks"))
	err := runner.Run(PreInstall, false)
	require.NoError(t, err) // Should not error on empty dir
}

func TestRun_SkipsMissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"))
	err := runner.Run(PostLink, false)
	require.NoError(t, err) // Should not error on missing dir
}

func TestRun_SkipsNonExecutableFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks", "post-link")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create executable script
	execScript := filepath.Join(hooksDir, "01-exec.sh")
	require.NoError(t, os.WriteFile(execScript, []byte("#!/bin/bash\necho exec >> "+outputFile), 0755))

	// Create non-executable file (like a README)
	readme := filepath.Join(hooksDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Hooks"), 0644))

	runner := New(filepath.Join(tmpDir, "hooks"))
	err := runner.Run(PostLink, false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "exec\n", string(content))
}

func TestListScripts(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks", "pre-install")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script1 := filepath.Join(hooksDir, "02-second.sh")
	require.NoError(t, os.WriteFile(script1, []byte("#!/bin/bash"), 0755))

	script2 := filepath.Join(hooksDir, "01-first.sh")
	require.NoError(t, os.WriteFile(script2, []byte("#!/bin/bash"), 0755))

	// Non-executable should not be listed
	readme := filepath.Join(hooksDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Hooks"), 0644))

	runner := New(filepath.Join(tmpDir, "hooks"))
	scripts, err := runner.ListScripts(PreInstall)
	require.NoError(t, err)

	require.Len(t, scripts, 2)
	assert.Equal(t, "01-first.sh", filepath.Base(scripts[0]))
	assert.Equal(t, "02-second.sh", filepath.Base(scripts[1]))
}

func TestHookType_String(t *testing.T) {
	assert.Equal(t, "pre-install", PreInstall.String())
	assert.Equal(t, "post-install", PostInstall.String())
	assert.Equal(t, "pre-link", PreLink.String())
	assert.Equal(t, "post-link", PostLink.String())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
