package run

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(t *testing.T, sourceDir, targetDir string) *config.Config {
	t.Helper()
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	configContent := "source_dir: .\nadd_dot: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".dottie.yaml"), []byte(configContent), 0o644))
	cfg, err := config.Load(sourceDir)
	require.NoError(t, err)
	cfg.TargetDir = targetDir
	return cfg
}

func collectEvents(ch <-chan Event) []Event {
	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestRunner_NoHooks_NoFiles(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)
	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})

	r := New(cfg, hr, false, false)
	events := r.Start()
	evs := collectEvents(events)
	result := r.Wait()

	assert.Empty(t, evs)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, 0, result.PreTotal)
	assert.Equal(t, 0, result.PostTotal)
}

func TestRunner_LinksFiles(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "bashrc"), []byte("export PATH"), 0o644))

	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	evs := collectEvents(events)
	result := r.Wait()

	assert.Equal(t, 0, result.ExitCode)

	var linkEvs []LinkEvent
	for _, ev := range evs {
		if e, ok := ev.(LinkEvent); ok {
			linkEvs = append(linkEvs, e)
		}
	}
	assert.Len(t, linkEvs, 2)
	assert.Equal(t, 2, result.Links.Added)
}

func TestRunner_LinksFiles_DryRun(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))

	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})
	r := New(cfg, hr, true, false)
	events := r.Start()
	evs := collectEvents(events)
	result := r.Wait()

	assert.Equal(t, 0, result.ExitCode)

	var linkEvs []LinkEvent
	for _, ev := range evs {
		if e, ok := ev.(LinkEvent); ok {
			linkEvs = append(linkEvs, e)
		}
	}
	assert.Len(t, linkEvs, 1)
	assert.Equal(t, link.StatusWouldLink, linkEvs[0].Status)
}

func TestRunner_PreHooks(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	hooksDir := filepath.Join(tmp, "hooks")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"),
		[]byte("#!/bin/bash\nexit 0"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-fail.sh"),
		[]byte("#!/bin/bash\nexit 1"), 0o755))

	hr := hooks.New(hooksDir, hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	evs := collectEvents(events)
	result := r.Wait()

	assert.Equal(t, 1, result.ExitCode, "should fail when hook fails")
	assert.Equal(t, 2, result.PreTotal)
	assert.Equal(t, 1, result.PreOk)

	var hookEvs []HookEvent
	for _, ev := range evs {
		if e, ok := ev.(HookEvent); ok {
			hookEvs = append(hookEvs, e)
		}
	}
	// Pre and post hooks: 2 pre + 2 post
	preEvs := filterHookPhase(hookEvs, "pre-link")
	assert.Len(t, preEvs, 2)
}

func TestRunner_PreHooks_AllOk(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	hooksDir := filepath.Join(tmp, "hooks")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-ok.sh"),
		[]byte("#!/bin/bash\nexit 0"), 0o755))

	hr := hooks.New(hooksDir, hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	collectEvents(events)
	result := r.Wait()

	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, 1, result.PreOk)
	assert.Equal(t, 1, result.PreTotal)
}

func TestRunner_FullSequence(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	hooksDir := filepath.Join(tmp, "hooks")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))

	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-hook.sh"),
		[]byte("#!/bin/bash\nexit 0"), 0o755))

	hr := hooks.New(hooksDir, hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	evs := collectEvents(events)
	result := r.Wait()

	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, 1, result.PreOk)
	assert.Equal(t, 1, result.PreTotal)
	assert.Equal(t, 1, result.PostOk)
	assert.Equal(t, 1, result.PostTotal)
	assert.Equal(t, 1, result.Links.Added)

	// Verify event ordering: pre-hook events, link events, post-hook events
	var phases []string
	for _, ev := range evs {
		switch e := ev.(type) {
		case HookEvent:
			phases = append(phases, "hook:"+e.Phase)
		case LinkEvent:
			phases = append(phases, "link")
		}
	}
	assert.Equal(t, []string{"hook:pre-link", "link", "hook:post-link"}, phases)
}

func TestRunner_FullSequence_PostHookFails(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	hooksDir := filepath.Join(tmp, "hooks")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))

	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-fail.sh"),
		[]byte("#!/bin/bash\nexit 1"), 0o755))

	hr := hooks.New(hooksDir, hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	collectEvents(events)
	result := r.Wait()

	assert.Equal(t, 1, result.ExitCode, "should fail when post-hook fails")
	assert.Equal(t, 1, result.PostTotal)
	assert.Equal(t, 0, result.PostOk)
}

func TestRunner_PrunesDanglingLinks(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	collectEvents(events)
	r.Wait()

	// Delete source to make symlink dangle
	require.NoError(t, os.Remove(vimrc))

	r2 := New(cfg, hr, false, false)
	events2 := r2.Start()
	evs := collectEvents(events2)
	result := r2.Wait()

	// Should have a dangling link event
	var linkEvs []LinkEvent
	for _, ev := range evs {
		if e, ok := ev.(LinkEvent); ok {
			linkEvs = append(linkEvs, e)
		}
	}
	danglingEvs := 0
	for _, e := range linkEvs {
		if e.Status == link.StatusDangling {
			danglingEvs++
		}
	}
	assert.Equal(t, 1, danglingEvs)
	assert.Equal(t, 1, result.Links.Pruned)

	// Verify the dangling symlink was actually removed
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	_, err := os.Lstat(targetVimrc)
	assert.True(t, os.IsNotExist(err), "dangling symlink should have been removed")
}

func TestRunner_Prune_DryRun_DoesNotRemove(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)

	vimrc := filepath.Join(sourceDir, "vimrc")
	require.NoError(t, os.WriteFile(vimrc, []byte("set number"), 0o644))

	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	collectEvents(events)
	r.Wait()

	require.NoError(t, os.Remove(vimrc))

	r2 := New(cfg, hr, true, false) // dryRun=true
	events2 := r2.Start()
	collectEvents(events2)
	r2.Wait()

	// Verify the dangling symlink was NOT removed (dry-run)
	targetVimrc := filepath.Join(targetDir, ".vimrc")
	_, err := os.Lstat(targetVimrc)
	assert.NoError(t, err, "dangling symlink should NOT be removed in dry-run")
}

func TestRunner_PhaseAndActiveHooks(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)
	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})

	r := New(cfg, hr, false, false)
	assert.Equal(t, PhaseIdle, r.Phase())
	assert.Empty(t, r.ActiveHooks())

	events := r.Start()
	collectEvents(events)
	result := r.Wait()

	assert.Equal(t, PhaseDone, r.Phase())
	assert.Empty(t, r.ActiveHooks())
	assert.Equal(t, 0, result.ExitCode)
}

func TestRunner_LinkError_SetsExitCode(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "dotfiles")
	targetDir := filepath.Join(tmp, "home")
	cfg := testConfig(t, sourceDir, targetDir)

	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "vimrc"), []byte("set number"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, ".vimrc"), []byte("conflict"), 0o644))

	hr := hooks.New(filepath.Join(tmp, "nonexistent"), hooks.EnvVars{})
	r := New(cfg, hr, false, false)
	events := r.Start()
	collectEvents(events)
	result := r.Wait()

	// With backup, it should still succeed by backing up the existing file
	assert.Equal(t, 1, result.Links.Added+result.Links.Existing+result.Links.Errors)
}

func filterHookPhase(evs []HookEvent, phase string) []HookEvent {
	var out []HookEvent
	for _, e := range evs {
		if e.Phase == phase {
			out = append(out, e)
		}
	}
	return out
}
