package hooks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/clutchski/dottie/internal/util"
)

// EnvVars holds environment variables passed to hook scripts.
type EnvVars map[string]string

// Runner executes hook scripts.
type Runner struct {
	hooksDir string
	env      EnvVars

	mu     sync.RWMutex
	active map[string]struct{}
}

// New creates a new hook Runner.
// hooksDir is the path to the hooks directory.
// env is additional environment variables set on each hook process.
func New(hooksDir string, env EnvVars) *Runner {
	return &Runner{
		hooksDir: hooksDir,
		env:      env,
	}
}

// Result is the outcome of running a single hook.
type Result struct {
	Name     string
	ExitCode int
	Elapsed  time.Duration
	Output   string // captured stdout+stderr
}

// Status represents the interpreted outcome of a hook.
type Status int

const (
	StatusOk          Status = iota // exit 0
	StatusNeedsUpdate               // exit 1 (meaningful for status-phase hooks)
	StatusFailed                    // exit 2+
)

// Ok returns true if the hook exited successfully.
func (r Result) Ok() bool { return r.ExitCode == 0 }

// Status returns the interpreted status based on exit code.
// For status-phase hooks: Ok, NeedsUpdate, or Failed.
// For run-phase hooks: callers typically just use Ok().
func (r Result) Status() Status {
	if r.ExitCode == 0 {
		return StatusOk
	}
	if r.ExitCode == 1 {
		return StatusNeedsUpdate
	}
	return StatusFailed
}

// StatusResult holds the collected results of an async status hooks run.
type StatusResult struct {
	Results []Result
	Err     error
}

// RunStatusAsync runs status hooks in the background and returns a channel
// that receives the collected results when all hooks complete.
func (r *Runner) RunStatusAsync() <-chan StatusResult {
	ch := make(chan StatusResult, 1)
	go func() {
		results, err := r.RunPhase("status", false)
		if err != nil {
			ch <- StatusResult{Err: err}
			return
		}
		var collected []Result
		for res := range results {
			collected = append(collected, res)
		}
		ch <- StatusResult{Results: collected}
	}()
	return ch
}

// RunPhase runs all hooks for a phase in parallel, streaming results on a channel.
// The channel closes when all hooks complete.
func (r *Runner) RunPhase(phase string, dryRun bool) (<-chan Result, error) {
	scripts, err := r.List()
	if err != nil {
		return nil, err
	}
	return r.RunScripts(scripts, phase, dryRun), nil
}

// Active returns the display names of currently running hooks.
func (r *Runner) Active() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.active))
	for name := range r.active {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RunScripts runs the given hook scripts for a phase in parallel, streaming
// results on a channel. The channel closes when all hooks complete.
// All scripts are marked active upfront (before goroutines start) so that
// callers like the spinner can display them immediately.
func (r *Runner) RunScripts(scripts []string, phase string, dryRun bool) <-chan Result {
	ch := make(chan Result, 16)

	r.mu.Lock()
	r.active = make(map[string]struct{}, len(scripts))
	for _, s := range scripts {
		r.active[DisplayName(filepath.Base(s))] = struct{}{}
	}
	r.mu.Unlock()

	go func() {
		defer close(ch)

		if len(scripts) == 0 {
			return
		}

		var wg sync.WaitGroup
		for _, script := range scripts {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				result := r.runHook(path, phase, dryRun)

				r.mu.Lock()
				delete(r.active, result.Name)
				r.mu.Unlock()

				ch <- result
			}(script)
		}
		wg.Wait()

		r.mu.Lock()
		r.active = nil
		r.mu.Unlock()
	}()

	return ch
}

func (r *Runner) runHook(path, phase string, dryRun bool) Result {
	start := time.Now()
	exitCode, output := r.execScript(path, phase, boolToString(dryRun))
	return Result{
		Name:     DisplayName(filepath.Base(path)),
		ExitCode: exitCode,
		Elapsed:  time.Since(start),
		Output:   output,
	}
}

// List returns all active hook scripts (executable, non-hidden, non-example),
// sorted alphabetically.
func (r *Runner) List() ([]string, error) {
	if _, err := os.Stat(r.hooksDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(r.hooksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read hooks directory: %w", err)
	}

	var scripts []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden files (like .gitkeep)
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Skip example files (like hook.example.sh)
		if strings.HasSuffix(name, ".example.sh") {
			continue
		}

		path := filepath.Join(r.hooksDir, name)
		if !util.IsExecutable(path) {
			continue
		}

		scripts = append(scripts, path)
	}

	sort.Strings(scripts)
	return scripts, nil
}

// execScript runs a hook script with the given phase arg and dry-run env value,
// returning the exit code and captured stdout+stderr.
func (r *Runner) execScript(path, phase, dryRun string) (int, string) {
	cmd := exec.Command(path, phase)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = r.buildEnv("DOTTIE_DRY_RUN", dryRun)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return exitCode, buf.String()
}

// buildEnv returns os.Environ() plus the runner's env vars and any extra key=value pairs.
func (r *Runner) buildEnv(extra ...string) []string {
	env := os.Environ()
	for k, v := range r.env {
		env = append(env, k+"="+v)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		env = append(env, extra[i]+"="+extra[i+1])
	}
	return env
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// DisplayName strips the file extension (e.g. "01-homebrew.sh" -> "01-homebrew").
func DisplayName(name string) string {
	if ext := filepath.Ext(name); ext != name {
		return strings.TrimSuffix(name, ext)
	}
	return name
}
