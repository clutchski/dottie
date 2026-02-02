package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// HookType represents when a hook runs.
type HookType int

const (
	PreInstall HookType = iota
	PostInstall
	PreLink
	PostLink
)

func (h HookType) String() string {
	switch h {
	case PreInstall:
		return "pre-install"
	case PostInstall:
		return "post-install"
	case PreLink:
		return "pre-link"
	case PostLink:
		return "post-link"
	default:
		return "unknown"
	}
}

// Runner executes hook scripts.
type Runner struct {
	hooksDir string
}

// New creates a new hook Runner.
func New(hooksDir string) *Runner {
	return &Runner{hooksDir: hooksDir}
}

// Run executes all scripts for the given hook type.
// Scripts are run in alphabetical order.
// If dryRun is true, scripts are listed but not executed.
func (r *Runner) Run(hookType HookType, dryRun bool) error {
	scripts, err := r.ListScripts(hookType)
	if err != nil {
		return err
	}

	if len(scripts) == 0 {
		return nil
	}

	for _, script := range scripts {
		if dryRun {
			fmt.Printf("[dry-run] would run: %s\n", script)
			continue
		}

		if err := r.runScript(script); err != nil {
			return fmt.Errorf("hook %s failed: %w", filepath.Base(script), err)
		}
	}

	return nil
}

// ListScripts returns all executable scripts for the given hook type,
// sorted alphabetically.
func (r *Runner) ListScripts(hookType HookType) ([]string, error) {
	hookDir := filepath.Join(r.hooksDir, hookType.String())

	if _, err := os.Stat(hookDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(hookDir)
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
		if name[0] == '.' {
			continue
		}

		path := filepath.Join(hookDir, name)
		if !isExecutable(path) {
			continue
		}

		scripts = append(scripts, path)
	}

	sort.Strings(scripts)
	return scripts, nil
}

func (r *Runner) runScript(path string) error {
	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

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
