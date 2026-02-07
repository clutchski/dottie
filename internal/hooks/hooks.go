package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EventKind identifies what happened during hook execution.
type EventKind int

const (
	Started EventKind = iota
	Done
)

// Event is a lifecycle event emitted as hooks run.
// For Done events, Err is nil on success or non-nil on failure.
// Stdout and Stderr are populated on Done events with captured hook output.
type Event struct {
	Kind     EventKind
	Hook     string        // full path to the hook script
	Phase    string        // "pre-link", "post-link", or "status"
	Duration time.Duration // set on Done
	Err      error         // set on Done if hook failed
	Stdout   []byte        // set on Done
	Stderr   []byte        // set on Done
}

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

// RunPreLink runs all hooks with "pre-link" phase in parallel.
// Returns a channel of lifecycle events. The channel closes when all hooks complete.
func (r *Runner) RunPreLink(dryRun bool) (<-chan Event, error) {
	return r.run("pre-link", dryRun)
}

// RunPostLink runs all hooks with "post-link" phase in parallel.
func (r *Runner) RunPostLink(dryRun bool) (<-chan Event, error) {
	return r.run("post-link", dryRun)
}

// CheckStatus runs all hooks with "status" phase in parallel.
func (r *Runner) CheckStatus() (<-chan Event, error) {
	return r.run("status", false)
}

func (r *Runner) run(phase string, dryRun bool) (<-chan Event, error) {
	scripts, err := r.List()
	if err != nil {
		return nil, err
	}

	if len(scripts) == 0 {
		ch := make(chan Event)
		close(ch)
		return ch, nil
	}

	events := make(chan Event, 2*len(scripts))

	var wg sync.WaitGroup
	for _, script := range scripts {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			events <- Event{Kind: Started, Hook: path, Phase: phase}

			start := time.Now()
			var stdout, stderr bytes.Buffer
			err := r.runScriptCapture(path, phase, dryRun, &stdout, &stderr)
			dur := time.Since(start)

			events <- Event{Kind: Done, Hook: path, Phase: phase, Duration: dur, Err: err, Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
		}(script)
	}

	go func() {
		wg.Wait()
		close(events)
	}()

	return events, nil
}

func (r *Runner) runScriptCapture(path, phase string, dryRun bool, stdout, stderr *bytes.Buffer) error {
	cmd := exec.Command(path, phase)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
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

