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
)

// EnvVars holds environment variables passed to hook scripts.
type EnvVars map[string]string

// Runner executes hook scripts.
type Runner struct {
	hooksDir string
	env      EnvVars
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

// HookResult is the outcome of running a single hook.
type HookResult struct {
	Name     string
	ExitCode int
	Elapsed  time.Duration
	Output   string // captured stdout+stderr
}

// Ok returns true if the hook exited successfully.
func (r HookResult) Ok() bool { return r.ExitCode == 0 }

// RunPhase runs all hooks for a phase in parallel, streaming results on a channel.
// The channel closes when all hooks complete.
func (r *Runner) RunPhase(phase string, dryRun bool) <-chan HookResult {
	ch := make(chan HookResult, 16)

	go func() {
		defer close(ch)

		scripts, err := r.List()
		if err != nil || len(scripts) == 0 {
			return
		}

		var wg sync.WaitGroup
		for _, script := range scripts {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				ch <- r.runHook(path, phase, dryRun)
			}(script)
		}
		wg.Wait()
	}()

	return ch
}

func (r *Runner) runHook(path, phase string, dryRun bool) HookResult {
	name := DisplayName(filepath.Base(path))
	start := time.Now()

	cmd := exec.Command(path, phase)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = r.buildEnv("DOTTIE_DRY_RUN", boolToString(dryRun))

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return HookResult{
		Name:     name,
		ExitCode: exitCode,
		Elapsed:  time.Since(start),
		Output:   buf.String(),
	}
}

// HookStatus represents the status of a single hook.
type HookStatus struct {
	Name     string
	ExitCode int
}

// Ok returns true if the hook exited successfully.
func (s HookStatus) Ok() bool { return s.ExitCode == 0 }

// StatusResult holds the results of hook status checks.
type StatusResult struct {
	Hooks []HookStatus
	Err   error
}

// StartStatusCheck begins running all hooks with "status" phase in parallel.
// Returns a channel that will receive the results when complete.
func (r *Runner) StartStatusCheck() <-chan StatusResult {
	ch := make(chan StatusResult, 1)

	go func() {
		scripts, err := r.List()
		if err != nil {
			ch <- StatusResult{Err: err}
			return
		}

		if len(scripts) == 0 {
			ch <- StatusResult{Hooks: nil}
			return
		}

		statuses := make([]HookStatus, len(scripts))
		var wg sync.WaitGroup
		for i, script := range scripts {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				exitCode := r.runStatusScript(path)
				statuses[idx] = HookStatus{
					Name:     filepath.Base(path),
					ExitCode: exitCode,
				}
			}(i, script)
		}
		wg.Wait()

		ch <- StatusResult{Hooks: statuses}
	}()

	return ch
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
		if !isExecutable(path) {
			continue
		}

		scripts = append(scripts, path)
	}

	sort.Strings(scripts)
	return scripts, nil
}

// runStatusScript runs a hook with "status" phase silently and returns the exit code.
func (r *Runner) runStatusScript(path string) int {
	cmd := exec.Command(path, "status")
	cmd.Env = r.buildEnv("DOTTIE_DRY_RUN", "false")

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
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

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
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
