package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

// ANSI color codes
const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
)

// Run executes all hooks with the given phase as the first argument.
// Phase should be one of: "pre-link", "post-link", "status".
// Scripts are run in alphabetical order.
// Environment variables DOTTIE_ROOT, DOTTIE_HOME, and DOTTIE_DRY_RUN are set.
func (r *Runner) Run(phase string, dryRun bool) error {
	scripts, err := r.List()
	if err != nil {
		return err
	}

	if len(scripts) == 0 {
		return nil
	}

	for _, script := range scripts {
		if err := r.runScript(script, phase, dryRun); err != nil {
			return fmt.Errorf("hook %s failed: %w", filepath.Base(script), err)
		}
	}

	return nil
}

// RunStatus executes all hooks with "status" phase and formats output.
// Hooks return exit code 0 for ok, non-zero for needs update.
// Returns true if all hooks are ok.
func (r *Runner) RunStatus() (bool, error) {
	scripts, err := r.List()
	if err != nil {
		return false, err
	}

	if len(scripts) == 0 {
		return true, nil
	}

	fmt.Println("Hooks:")
	allOk := true
	for _, script := range scripts {
		name := filepath.Base(script)

		ok := r.runStatusScript(script)
		if ok {
			fmt.Printf("  %s[✓]%s %s\n", colorGreen, colorReset, name)
		} else {
			fmt.Printf("  %s[x]%s %s\n", colorRed, colorReset, name)
			allOk = false
		}
	}

	return allOk, nil
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

func (r *Runner) runScript(path, phase string, dryRun bool) error {
	cmd := exec.Command(path, phase)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

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
