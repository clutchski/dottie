package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"github.com/clutchski/dottie/internal/console"
)

// Runner executes hook scripts.
type Runner struct {
	hooksDir string
	rootDir  string
	homeDir  string
}

// New creates a new hook Runner.
// hooksDir is the path to the hooks directory.
// rootDir is the dotfiles repository root (set as DOTTIE_ROOT).
// homeDir is the target home directory (set as DOTTIE_HOME).
func New(hooksDir, rootDir, homeDir string) *Runner {
	return &Runner{
		hooksDir: hooksDir,
		rootDir:  rootDir,
		homeDir:  homeDir,
	}
}

// Run executes all hooks with the given phase in parallel.
// Phase should be one of: "pre-link", "post-link", "status".
// Output is written to con.Stdout and con.Stderr.
// Environment variables DOTTIE_ROOT, DOTTIE_HOME, and DOTTIE_DRY_RUN are set.
func (r *Runner) Run(phase string, dryRun, verbose bool, con *console.Console) error {
	scripts, err := r.List()
	if err != nil {
		return err
	}

	if len(scripts) == 0 {
		return nil
	}

	if verbose {
		fmt.Fprintf(con.Stdout, "\nhooks %s:\n", phase)
	}

	// Run all hooks in parallel
	errors := make([]error, len(scripts))
	var wg sync.WaitGroup
	for i, script := range scripts {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			name := DisplayName(filepath.Base(path))
			if verbose {
				fmt.Fprintf(con.Stdout, "  \033[33m◎\033[0m start: %s\n", name)
			}
			start := time.Now()
			if err := r.runScript(path, phase, dryRun, con); err != nil {
				errors[idx] = fmt.Errorf("hook %s failed: %w", name, err)
				if verbose {
					fmt.Fprintf(con.Stdout, "  \033[31mx\033[0m done:  %s (%.1fs)\n", name, time.Since(start).Seconds())
				}
			} else if verbose {
				fmt.Fprintf(con.Stdout, "  \033[32m✓\033[0m done:  %s (%.1fs)\n", name, time.Since(start).Seconds())
			}
		}(i, script)
	}
	wg.Wait()

	// Return first error if any
	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// RunAll executes all hooks for a phase and returns per-hook results.
// Unlike Run, it does not stop on the first error.
func (r *Runner) RunAll(phase string, dryRun, verbose bool, con *console.Console) ([]HookStatus, error) {
	scripts, err := r.List()
	if err != nil {
		return nil, err
	}

	if len(scripts) == 0 {
		return nil, nil
	}

	if verbose {
		fmt.Fprintf(con.Stdout, "\nhooks %s:\n", phase)
	}

	statuses := make([]HookStatus, len(scripts))
	var wg sync.WaitGroup
	for i, script := range scripts {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			baseName := filepath.Base(path)
			name := DisplayName(baseName)
			if verbose {
				fmt.Fprintf(con.Stdout, "  \033[33m◎\033[0m start: %s\n", name)
			}
			start := time.Now()
			err := r.runScript(path, phase, dryRun, con)
			statuses[idx] = HookStatus{Name: baseName, Ok: err == nil}
			if verbose {
				if err != nil {
					fmt.Fprintf(con.Stdout, "  \033[31mx\033[0m done:  %s (%.1fs)\n", name, time.Since(start).Seconds())
				} else {
					fmt.Fprintf(con.Stdout, "  \033[32m✓\033[0m done:  %s (%.1fs)\n", name, time.Since(start).Seconds())
				}
			}
		}(i, script)
	}
	wg.Wait()

	return statuses, nil
}

// HookStatus represents the status of a single hook.
type HookStatus struct {
	Name string
	Ok   bool
}

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

		// Run all hooks in parallel
		statuses := make([]HookStatus, len(scripts))
		var wg sync.WaitGroup
		for i, script := range scripts {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				statuses[idx] = HookStatus{
					Name: filepath.Base(path),
					Ok:   r.runStatusScript(path),
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

// runStatusScript runs a hook with "status" phase silently and returns true if exit code is 0.
func (r *Runner) runStatusScript(path string) bool {
	cmd := exec.Command(path, "status")

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"DOTTIE_ROOT="+r.rootDir,
		"DOTTIE_HOME="+r.homeDir,
		"DOTTIE_DRY_RUN=false",
	)

	// Run silently - we only care about exit code
	err := cmd.Run()
	return err == nil
}

func (r *Runner) runScript(path, phase string, dryRun bool, con *console.Console) error {
	cmd := exec.Command(path, phase)
	cmd.Stdout = con.Stdout
	cmd.Stderr = con.Stderr

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"DOTTIE_ROOT="+r.rootDir,
		"DOTTIE_HOME="+r.homeDir,
		"DOTTIE_DRY_RUN="+boolToString(dryRun),
	)

	return cmd.Run()
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if any execute bit is set
	return info.Mode()&0111 != 0
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
