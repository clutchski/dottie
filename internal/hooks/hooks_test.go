package hooks

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var emptyEnvVars map[string]string

func TestRunPreLink_ReturnsEvents(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	scriptPath := filepath.Join(hooksDir, "01-hello.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(`#!/bin/bash
echo "hello stdout"
echo "hello stderr" >&2`), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)

	var started, done int
	for ev := range events {
		assert.Equal(t, scriptPath, ev.Hook)
		assert.Equal(t, "pre-link", ev.Phase)
		switch ev.Kind {
		case Started:
			started++
		case Done:
			done++
			assert.NoError(t, ev.Err)
			assert.Greater(t, ev.Duration, time.Duration(0))
			assert.Contains(t, string(ev.Stdout), "hello stdout")
			assert.Contains(t, string(ev.Stderr), "hello stderr")
		}
	}
	assert.Equal(t, 1, started)
	assert.Equal(t, 1, done)
}

func TestRunPreLink_FailedHook(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "fail.sh"), []byte(`#!/bin/bash
echo "failing" >&2
exit 1`), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)

	var doneEvents []Event
	for ev := range events {
		if ev.Kind == Done {
			doneEvents = append(doneEvents, ev)
		}
	}
	require.Len(t, doneEvents, 1)
	assert.Error(t, doneEvents[0].Err)
	assert.Contains(t, string(doneEvents[0].Stderr), "failing")
}

func TestRunPreLink_ExecutesAllScripts(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-first.sh"), []byte(`#!/bin/bash
echo "first" >> `+outputFile), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-second.sh"), []byte(`#!/bin/bash
echo "second" >> `+outputFile), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "first")
	assert.Contains(t, string(content), "second")
}

func TestRunPreLink_PassesPhaseAsArgument(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hook.sh"), []byte(`#!/bin/bash
echo "$1" >> `+outputFile), 0755))

	runner := New(hooksDir, emptyEnvVars)

	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for range events {
	}

	events, err = runner.RunPostLink(false)
	require.NoError(t, err)
	for range events {
	}

	events, err = runner.CheckStatus()
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "pre-link\npost-link\nstatus\n", string(content))
}

func TestRunPreLink_SetsEnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "check-env.sh"), []byte(`#!/bin/bash
echo "ROOT=$DOTTIE_ROOT" >> `+outputFile+`
echo "HOME=$DOTTIE_HOME" >> `+outputFile+`
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0755))

	runner := New(hooksDir, map[string]string{
		"DOTTIE_ROOT": "/my/dotfiles",
		"DOTTIE_HOME": "/home/user",
	})
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ROOT=/my/dotfiles")
	assert.Contains(t, string(content), "HOME=/home/user")
	assert.Contains(t, string(content), "DRY_RUN=false")
}

func TestRunPreLink_DryRunSetsEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "check-dry-run.sh"), []byte(`#!/bin/bash
echo "DRY_RUN=$DOTTIE_DRY_RUN" >> `+outputFile), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(true)
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "DRY_RUN=true")
}

func TestRunPreLink_NoHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)

	count := 0
	for range events {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestRunPreLink_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)

	count := 0
	for range events {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestRunPreLink_SkipsHiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, ".hidden.sh"), []byte(`#!/bin/bash
echo "hidden" >> `+outputFile), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "normal.sh"), []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRunPreLink_SkipsExampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "homebrew.example.sh"), []byte(`#!/bin/bash
echo "example" >> `+outputFile), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "homebrew.sh"), []byte(`#!/bin/bash
echo "normal" >> `+outputFile), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "normal\n", string(content))
}

func TestRunPreLink_SkipsNonExecutableFiles(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	outputFile := filepath.Join(tmpDir, "output.txt")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "README.md"), []byte("# Hooks"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hook.sh"), []byte(`#!/bin/bash
echo "exec" >> `+outputFile), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for range events {
	}

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "exec\n", string(content))
}

func TestRunPreLink_SetsPhaseOnEvents(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hook.sh"), []byte("#!/bin/bash\nexit 0"), 0755))

	runner := New(hooksDir, emptyEnvVars)

	events, err := runner.RunPreLink(false)
	require.NoError(t, err)
	for ev := range events {
		assert.Equal(t, "pre-link", ev.Phase)
	}

	events, err = runner.RunPostLink(false)
	require.NoError(t, err)
	for ev := range events {
		assert.Equal(t, "post-link", ev.Phase)
	}

	events, err = runner.CheckStatus()
	require.NoError(t, err)
	for ev := range events {
		assert.Equal(t, "status", ev.Phase)
	}
}

func TestCheckStatus_MixedResults(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"), []byte("#!/bin/bash\nexit 0"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"), []byte("#!/bin/bash\nexit 1"), 0755))

	runner := New(hooksDir, emptyEnvVars)
	events, err := runner.CheckStatus()
	require.NoError(t, err)

	results := map[string]error{}
	for ev := range events {
		if ev.Kind == Done {
			results[filepath.Base(ev.Hook)] = ev.Err
		}
	}
	assert.NoError(t, results["01-ok.sh"])
	assert.Error(t, results["02-fail.sh"])
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

	runner := New(hooksDir, emptyEnvVars)
	hooks, err := runner.List()
	require.NoError(t, err)

	require.Len(t, hooks, 2)
	assert.Equal(t, "01-first.sh", filepath.Base(hooks[0]))
	assert.Equal(t, "02-second.sh", filepath.Base(hooks[1]))
}

func TestList_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runner := New(filepath.Join(tmpDir, "nonexistent"), emptyEnvVars)
	hooks, err := runner.List()
	require.NoError(t, err)
	assert.Empty(t, hooks)
}
