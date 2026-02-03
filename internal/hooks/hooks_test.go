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
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create scripts that append to a file based on phase argument
	script1 := filepath.Join(hooksDir, "01-first.sh")
	require.NoError(t, os.WriteFile(script1, []byte(`#!/bin/bash
echo "first:$1" >> `+outputFile), 0755))

	script2 := filepath.Join(hooksDir, "02-second.sh")
	require.NoError(t, os.WriteFile(script2, []byte(`#!/bin/bash
echo "second:$1" >> `+outputFile), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	err := runner.Run("pre-link", false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "first:pre-link\nsecond:pre-link\n", string(content))
}

func TestRun_PassesPhaseAsArgument(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "hook.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "$1" >> `+outputFile), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")

	// Test different phases
	require.NoError(t, runner.Run("pre-link", false))
	require.NoError(t, runner.Run("post-link", false))
	require.NoError(t, runner.Run("status", false))

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "pre-link\npost-link\nstatus\n", string(content))
}

func TestRun_SetsEnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "check-env.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "ROOT=$DOTTIE_ROOT" >> `+outputFile+`
echo "HOME=$DOTTIE_HOME" >> `+outputFile+`
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0755))

	runner := New(hooksDir, "/my/dotfiles", "/home/user")
	err := runner.Run("pre-link", false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ROOT=/my/dotfiles")
	assert.Contains(t, string(content), "HOME=/home/user")
	assert.Contains(t, string(content), "DRY_RUN=false")
}

func TestRun_DryRunSetsEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Script should still run in dry-run mode but with DOTTIE_DRY_RUN=true
	script := filepath.Join(hooksDir, "check-dry-run.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	err := runner.Run("pre-link", true) // dry-run
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "DRY_RUN=true")
}

func TestRun_SkipsMissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), tmpDir, "/home/test")
	err := runner.Run("pre-link", false)
	require.NoError(t, err) // Should not error on missing dir
}

func TestRun_SkipsHiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Hidden file should be skipped
	hidden := filepath.Join(hooksDir, ".hidden.sh")
	require.NoError(t, os.WriteFile(hidden, []byte(`#!/bin/bash
echo "hidden" >> `+outputFile), 0755))

	// Normal file should run
	normal := filepath.Join(hooksDir, "normal.sh")
	require.NoError(t, os.WriteFile(normal, []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	err := runner.Run("pre-link", false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRun_SkipsExampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Example file should be skipped
	example := filepath.Join(hooksDir, "homebrew.example.sh")
	require.NoError(t, os.WriteFile(example, []byte(`#!/bin/bash
echo "example" >> `+outputFile), 0755))

	// Normal file should run
	normal := filepath.Join(hooksDir, "homebrew.sh")
	require.NoError(t, os.WriteFile(normal, []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	err := runner.Run("pre-link", false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRun_SkipsNonExecutableFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Non-executable file should be skipped
	readme := filepath.Join(hooksDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Hooks"), 0644))

	// Executable file should run
	exec := filepath.Join(hooksDir, "hook.sh")
	require.NoError(t, os.WriteFile(exec, []byte(`#!/bin/bash
echo "exec" >> `+outputFile), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	err := runner.Run("pre-link", false)
	require.NoError(t, err)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "exec\n", string(content))
}

func TestList_ReturnsActiveHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create various files
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-second.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-first.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, ".hidden.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hook.example.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "README.md"), []byte("# Hooks"), 0644))

	runner := New(hooksDir, tmpDir, "/home/test")
	hooks, err := runner.List()
	require.NoError(t, err)

	require.Len(t, hooks, 2)
	assert.Equal(t, "01-first.sh", filepath.Base(hooks[0]))
	assert.Equal(t, "02-second.sh", filepath.Base(hooks[1]))
}

func TestList_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), tmpDir, "/home/test")
	hooks, err := runner.List()
	require.NoError(t, err)
	assert.Empty(t, hooks)
}

func TestRunStatus_AllOk(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create hooks that exit 0
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "ok-hook.sh"), []byte(`#!/bin/bash
exit 0`), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	allOk, err := runner.RunStatus()
	require.NoError(t, err)
	assert.True(t, allOk)
}

func TestRunStatus_SomeFailed(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create one hook that exits 0 and one that exits 1
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"), []byte(`#!/bin/bash
exit 0`), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"), []byte(`#!/bin/bash
exit 1`), 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	allOk, err := runner.RunStatus()
	require.NoError(t, err)
	assert.False(t, allOk)
}

func TestRunStatus_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	runner := New(hooksDir, tmpDir, "/home/test")
	allOk, err := runner.RunStatus()
	require.NoError(t, err)
	assert.True(t, allOk) // No hooks means all ok
}

func TestRunStatus_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), tmpDir, "/home/test")
	allOk, err := runner.RunStatus()
	require.NoError(t, err)
	assert.True(t, allOk) // No hooks directory means all ok
}
