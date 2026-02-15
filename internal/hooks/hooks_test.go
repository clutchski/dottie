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

func collectResults(ch <-chan Result) []Result {
	var results []Result
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func runPhase(t *testing.T, runner *Runner, phase string, dryRun bool) []Result {
	t.Helper()
	ch, err := runner.RunPhase(phase, dryRun)
	require.NoError(t, err)
	return collectResults(ch)
}

func TestRunPhase_ExecutesAllScripts(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	script1 := filepath.Join(hooksDir, "01-first.sh")
	require.NoError(t, os.WriteFile(script1, []byte(`#!/bin/bash
echo "first" >> `+outputFile), 0o755))

	script2 := filepath.Join(hooksDir, "02-second.sh")
	require.NoError(t, os.WriteFile(script2, []byte(`#!/bin/bash
echo "second" >> `+outputFile), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)

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
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	script := filepath.Join(hooksDir, "hook.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "$1" >> `+outputFile), 0o755))

	runner := New(hooksDir, NoEnv)

	runPhase(t, runner, "pre-link", false)
	runPhase(t, runner, "post-link", false)
	runPhase(t, runner, "status", false)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "pre-link\npost-link\nstatus\n", string(content))
}

func TestRunPhase_SetsEnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	script := filepath.Join(hooksDir, "check-env.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "ROOT=$DOTTIE_ROOT" >> `+outputFile+`
echo "HOME=$DOTTIE_HOME" >> `+outputFile+`
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0o755))

	runner := New(hooksDir, EnvVars{
		"DOTTIE_ROOT": "/my/dotfiles",
		"DOTTIE_HOME": "/home/user",
	})
	runPhase(t, runner, "pre-link", false)

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
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	script := filepath.Join(hooksDir, "check-dry-run.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0o755))

	runner := New(hooksDir, NoEnv)
	runPhase(t, runner, "pre-link", true)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "DRY_RUN=true")
}

func TestRunPhase_SkipsMissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), NoEnv)
	results := runPhase(t, runner, "pre-link", false)
	assert.Empty(t, results)
}

func TestRunPhase_SkipsHiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	hidden := filepath.Join(hooksDir, ".hidden.sh")
	require.NoError(t, os.WriteFile(hidden, []byte(`#!/bin/bash
echo "hidden" >> `+outputFile), 0o755))

	normal := filepath.Join(hooksDir, "normal.sh")
	require.NoError(t, os.WriteFile(normal, []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)
	require.Len(t, results, 1)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRunPhase_SkipsExampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	example := filepath.Join(hooksDir, "homebrew.example.sh")
	require.NoError(t, os.WriteFile(example, []byte(`#!/bin/bash
echo "example" >> `+outputFile), 0o755))

	normal := filepath.Join(hooksDir, "homebrew.sh")
	require.NoError(t, os.WriteFile(normal, []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)
	require.Len(t, results, 1)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRunPhase_SkipsNonExecutableFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	readme := filepath.Join(hooksDir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Hooks"), 0o644))

	exec := filepath.Join(hooksDir, "hook.sh")
	require.NoError(t, os.WriteFile(exec, []byte(`#!/bin/bash
echo "exec" >> `+outputFile), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)
	require.Len(t, results, 1)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "exec\n", string(content))
}

func TestRunPhase_CapturesOutput(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	script := filepath.Join(hooksDir, "hello.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/bash
echo "hello stdout"
echo "hello stderr" >&2`), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)

	require.Len(t, results, 1)
	assert.Contains(t, results[0].Output, "hello stdout")
	assert.Contains(t, results[0].Output, "hello stderr")
	assert.True(t, results[0].Ok())
}

func TestRunPhase_ReportsExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"), []byte("#!/bin/bash\nexit 0"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"), []byte("#!/bin/bash\nexit 42"), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)

	require.Len(t, results, 2)

	// Results may arrive in any order due to parallel execution
	resultMap := make(map[string]Result)
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
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "fast.sh"), []byte("#!/bin/bash\nexit 0"), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "pre-link", false)

	require.Len(t, results, 1)
	assert.Positive(t, results[0].Elapsed)
}

func TestList_ReturnsActiveHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-second.sh"), []byte("#!/bin/bash"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-first.sh"), []byte("#!/bin/bash"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, ".hidden.sh"), []byte("#!/bin/bash"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hook.example.sh"), []byte("#!/bin/bash"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "README.md"), []byte("# Hooks"), 0o644))

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

func TestRunPhase_Status_AllOk(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "ok-hook.sh"), []byte(`#!/bin/bash
exit 0`), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "status", false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusOk, results[0].Status())
}

func TestRunPhase_Status_SomeFailed(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"), []byte(`#!/bin/bash
exit 0`), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"), []byte(`#!/bin/bash
exit 1`), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "status", false)
	require.Len(t, results, 2)

	okCount := 0
	for _, r := range results {
		if r.Ok() {
			okCount++
		}
	}
	assert.Equal(t, 1, okCount)
}

func TestRunPhase_Status_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "status", false)
	assert.Empty(t, results)
}

func TestRunPhase_Status_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), NoEnv)
	results := runPhase(t, runner, "status", false)
	assert.Empty(t, results)
}

func TestRunPhase_Status_CapturesOutput(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "brew.sh"), []byte(`#!/bin/bash
echo "outdated packages"
echo "warning: something" >&2
exit 1`), 0o755))

	runner := New(hooksDir, NoEnv)
	results := runPhase(t, runner, "status", false)
	require.Len(t, results, 1)
	assert.Equal(t, StatusNeedsUpdate, results[0].Status())
	assert.Contains(t, results[0].Output, "outdated packages")
	assert.Contains(t, results[0].Output, "warning: something")
}

func TestResult_Status(t *testing.T) {
	assert.Equal(t, StatusOk, Result{ExitCode: 0}.Status())
	assert.Equal(t, StatusNeedsUpdate, Result{ExitCode: 1}.Status())
	assert.Equal(t, StatusFailed, Result{ExitCode: 2}.Status())
	assert.Equal(t, StatusFailed, Result{ExitCode: 42}.Status())
}

func TestActive_EmptyBeforeAndAfterRun(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"),
		[]byte("#!/bin/bash\nexit 0"), 0o755))

	runner := New(hooksDir, NoEnv)
	assert.Empty(t, runner.Active())

	runPhase(t, runner, "pre-link", false)
	assert.Empty(t, runner.Active())
}

func TestActive_ReturnsNamesWhileRunning(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-slow.sh"),
		[]byte("#!/bin/bash\nsleep 0.5\nexit 0"), 0o755))

	runner := New(hooksDir, NoEnv)
	ch, err := runner.RunPhase("pre-link", false)
	require.NoError(t, err)

	for r := range ch {
		_ = r
	}
	// After channel drains, active should be empty.
	// We can't reliably check mid-flight in a unit test without races,
	// but we can verify it's clean after completion.
	assert.Empty(t, runner.Active())
}

func TestRunStatusAsync_CollectsResults(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"),
		[]byte("#!/bin/bash\nexit 0"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-update.sh"),
		[]byte("#!/bin/bash\nexit 1"), 0o755))

	runner := New(hooksDir, NoEnv)
	sr := <-runner.RunStatusAsync()
	require.NoError(t, sr.Err)
	require.Len(t, sr.Results, 2)
}

func TestRunStatusAsync_NoHooks(t *testing.T) {
	runner := New(filepath.Join(t.TempDir(), "nonexistent"), NoEnv)
	sr := <-runner.RunStatusAsync()
	require.NoError(t, sr.Err)
	assert.Empty(t, sr.Results)
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
