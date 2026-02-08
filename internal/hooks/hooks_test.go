package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NoEnv is an empty set of environment variables for test readability.
var NoEnv = EnvVars{}

func collectResults(ch <-chan HookResult) []HookResult {
	var results []HookResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestRunPhase_ExecutesAllScripts(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script1 := filepath.Join(hooksDir, "01-first.sh")
	require.NoError(t, os.WriteFile(script1, []byte(`#!/bin/bash
echo "first" >> `+outputFile), 0755))

	script2 := filepath.Join(hooksDir, "02-second.sh")
	require.NoError(t, os.WriteFile(script2, []byte(`#!/bin/bash
echo "second" >> `+outputFile), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))

	require.Len(t, results, 2)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "first")
	assert.Contains(t, string(content), "second")
}

func TestRunPhase_PassesPhaseAsArgument(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "hook.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "$1" >> `+outputFile), 0755))

	runner := New(hooksDir, NoEnv)

	collectResults(runner.RunPhase("pre-link", false))
	collectResults(runner.RunPhase("post-link", false))
	collectResults(runner.RunPhase("status", false))

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "pre-link\npost-link\nstatus\n", string(content))
}

func TestRunPhase_SetsEnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "check-env.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "ROOT=$DOTTIE_ROOT" >> `+outputFile+`
echo "HOME=$DOTTIE_HOME" >> `+outputFile+`
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0755))

	runner := New(hooksDir, EnvVars{
		"DOTTIE_ROOT": "/my/dotfiles",
		"DOTTIE_HOME": "/home/user",
	})
	collectResults(runner.RunPhase("pre-link", false))

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ROOT=/my/dotfiles")
	assert.Contains(t, string(content), "HOME=/home/user")
	assert.Contains(t, string(content), "DRY_RUN=false")
}

func TestRunPhase_DryRunSetsEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "check-dry-run.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0755))

	runner := New(hooksDir, NoEnv)
	collectResults(runner.RunPhase("pre-link", true))

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "DRY_RUN=true")
}

func TestRunPhase_SkipsMissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))
	assert.Empty(t, results)
}

func TestRunPhase_SkipsHiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	hidden := filepath.Join(hooksDir, ".hidden.sh")
	require.NoError(t, os.WriteFile(hidden, []byte(`#!/bin/bash
echo "hidden" >> `+outputFile), 0755))

	normal := filepath.Join(hooksDir, "normal.sh")
	require.NoError(t, os.WriteFile(normal, []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))
	require.Len(t, results, 1)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRunPhase_SkipsExampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	example := filepath.Join(hooksDir, "homebrew.example.sh")
	require.NoError(t, os.WriteFile(example, []byte(`#!/bin/bash
echo "example" >> `+outputFile), 0755))

	normal := filepath.Join(hooksDir, "homebrew.sh")
	require.NoError(t, os.WriteFile(normal, []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))
	require.Len(t, results, 1)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRunPhase_SkipsNonExecutableFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	readme := filepath.Join(hooksDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Hooks"), 0644))

	exec := filepath.Join(hooksDir, "hook.sh")
	require.NoError(t, os.WriteFile(exec, []byte(`#!/bin/bash
echo "exec" >> `+outputFile), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))
	require.Len(t, results, 1)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "exec\n", string(content))
}

func TestRunPhase_CapturesOutput(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	script := filepath.Join(hooksDir, "hello.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "hello stdout"
echo "hello stderr" >&2`), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))

	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello stdout")
	assert.Contains(t, results[0].Output, "hello stderr")
	assert.True(t, results[0].Ok())
}

func TestRunPhase_ReportsExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"), []byte("#!/bin/bash\nexit 0"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"), []byte("#!/bin/bash\nexit 42"), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))

	require.Len(t, results, 2)

	// Results may arrive in any order due to parallel execution
	resultMap := make(map[string]HookResult)
	for _, r := range results {
		resultMap[r.Name] = r
	}

	assert.True(t, resultMap["01-ok"].Ok())
	assert.Equal(t, 0, resultMap["01-ok"].ExitCode)
	assert.False(t, resultMap["02-fail"].Ok())
	assert.Equal(t, 42, resultMap["02-fail"].ExitCode)
}

func TestRunPhase_RecordsElapsed(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "fast.sh"), []byte("#!/bin/bash\nexit 0"), 0755))

	runner := New(hooksDir, NoEnv)
	results := collectResults(runner.RunPhase("pre-link", false))

	require.Len(t, results, 1)
	assert.True(t, results[0].Elapsed > 0)
}

func TestList_ReturnsActiveHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-second.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-first.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, ".hidden.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hook.example.sh"), []byte("#!/bin/bash"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "README.md"), []byte("# Hooks"), 0644))

	runner := New(hooksDir, NoEnv)
	hooks, err := runner.List()
	require.NoError(t, err)

	require.Len(t, hooks, 2)
	assert.Equal(t, "01-first.sh", filepath.Base(hooks[0]))
	assert.Equal(t, "02-second.sh", filepath.Base(hooks[1]))
}

func TestList_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), NoEnv)
	hooks, err := runner.List()
	require.NoError(t, err)
	assert.Empty(t, hooks)
}

func TestStartStatusCheck_AllOk(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "ok-hook.sh"), []byte(`#!/bin/bash
exit 0`), 0755))

	runner := New(hooksDir, NoEnv)
	result := <-runner.StartStatusCheck()
	require.NoError(t, result.Err)
	require.Len(t, result.Hooks, 1)
	assert.True(t, result.Hooks[0].Ok())
	assert.Equal(t, 0, result.Hooks[0].ExitCode)
}

func TestStartStatusCheck_SomeFailed(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"), []byte(`#!/bin/bash
exit 0`), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"), []byte(`#!/bin/bash
exit 1`), 0755))

	runner := New(hooksDir, NoEnv)
	result := <-runner.StartStatusCheck()
	require.NoError(t, result.Err)
	require.Len(t, result.Hooks, 2)
	assert.True(t, result.Hooks[0].Ok())
	assert.False(t, result.Hooks[1].Ok())
}

func TestStartStatusCheck_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	runner := New(hooksDir, NoEnv)
	result := <-runner.StartStatusCheck()
	require.NoError(t, result.Err)
	assert.Empty(t, result.Hooks)
}

func TestStartStatusCheck_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), NoEnv)
	result := <-runner.StartStatusCheck()
	require.NoError(t, result.Err)
	assert.Empty(t, result.Hooks)
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"homebrew.sh", "homebrew"},
		{"01-homebrew.sh", "01-homebrew"},
		{"setup", "setup"},
		{"my-script.bash", "my-script"},
		{"", ""},
		{".hidden", ".hidden"},
		{"no-ext", "no-ext"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, DisplayName(tt.input))
		})
	}
}
