package status

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/util"
)

// FileStatus represents the link status of a dotfile.
type FileStatus int

const (
	FileStatusLinked FileStatus = iota
	FileStatusMissing
	FileStatusDiff
)

func (s FileStatus) String() string {
	switch s {
	case FileStatusLinked:
		return "linked"
	case FileStatusMissing:
		return "unlinked"
	case FileStatusDiff:
		return "diff"
	default:
		return "unknown"
	}
}

// DotfileStatus represents the status of a single dotfile.
type DotfileStatus struct {
	Name       string
	SourcePath string
	TargetPath string
	Status     FileStatus
	Message    string
}

// Checker checks the status of dotfiles.
type Checker struct {
	cfg *config.Config
}

// New creates a new status Checker.
func New(cfg *config.Config) *Checker {
	return &Checker{cfg: cfg}
}

// GetStatus returns the status of all dotfiles.
func (c *Checker) GetStatus() ([]DotfileStatus, error) {
	sourceDir := c.cfg.GetSourcePath()
	return c.getStatusDir(sourceDir, c.cfg.TargetDir, "", true)
}

// getStatusDir recursively gets status of files in sourceDir.
// prefix is used to build display names like "config/starship.toml".
func (c *Checker) getStatusDir(sourceDir, targetDir, prefix string, topLevel bool) ([]DotfileStatus, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory: %w", err)
	}

	var statuses []DotfileStatus

	for _, entry := range entries {
		name := entry.Name()

		if c.cfg.ShouldIgnore(name) {
			continue
		}

		sourcePath := filepath.Join(sourceDir, name)

		var targetPath string
		var displayName string
		if topLevel {
			targetPath = c.cfg.GetTargetPath(name)
			displayName = name
		} else {
			targetPath = filepath.Join(targetDir, name)
			displayName = filepath.Join(prefix, name)
		}

		// If source is a directory, always recurse to show individual files
		if entry.IsDir() {
			subStatuses, err := c.getStatusDir(sourcePath, targetPath, displayName, false)
			if err != nil {
				return statuses, err
			}
			statuses = append(statuses, subStatuses...)
			continue
		}

		status := c.checkFile(sourcePath, targetPath)
		status.Name = displayName
		status.SourcePath = sourcePath
		status.TargetPath = targetPath

		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (c *Checker) checkFile(source, target string) DotfileStatus {
	status := DotfileStatus{}

	// Check if target exists
	if !util.FileExists(target) {
		status.Status = FileStatusMissing
		status.Message = "not linked"
		return status
	}

	// Check if target resolves to source (handles both direct symlinks
	// and files accessed through symlinked parent directories)
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		status.Status = FileStatusDiff
		status.Message = "cannot resolve target path"
		return status
	}

	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		resolvedSource = source // use as-is if can't resolve
	}

	if resolvedTarget == resolvedSource {
		status.Status = FileStatusLinked
		status.Message = "linked"
		return status
	}

	// Not linked - check if it's a symlink pointing elsewhere or a regular file
	if util.IsSymlink(target) {
		linkTarget, _ := util.SymlinkTarget(target)
		status.Status = FileStatusDiff
		status.Message = fmt.Sprintf("symlink points to %s", linkTarget)
	} else {
		status.Status = FileStatusDiff
		status.Message = "exists but not linked to repo"
	}
	return status
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
)

// Check returns true if all dotfiles are in sync, false otherwise.
// Also returns the list of statuses for display.
func (c *Checker) Check() (bool, []DotfileStatus, error) {
	statuses, err := c.GetStatus()
	if err != nil {
		return false, nil, err
	}

	allOk := true
	for _, s := range statuses {
		if s.Status != FileStatusLinked {
			allOk = false
			break
		}
	}

	return allOk, statuses, nil
}

// Print prints the verbose status to stdout.
// Returns true if all dotfiles are in sync.
func (c *Checker) Print() (bool, error) {
	return c.PrintVerbose(os.Stdout)
}

// PrintVerbose prints the full per-file status listing.
// Returns true if all dotfiles are in sync.
func (c *Checker) PrintVerbose(w io.Writer) (bool, error) {
	allOk, statuses, err := c.Check()
	if err != nil {
		return false, err
	}

	if len(statuses) == 0 {
		return true, nil
	}

	// Calculate max target width for alignment
	maxWidth := 0
	for _, s := range statuses {
		displayTarget := c.formatTargetPath(s.TargetPath)
		if len(displayTarget) > maxWidth {
			maxWidth = len(displayTarget)
		}
	}

	fmt.Fprintln(w, "Dotfiles:")
	for _, s := range statuses {
		var symbol, color, reason string
		switch s.Status {
		case FileStatusLinked:
			symbol = "✓"
			color = colorGreen
		case FileStatusMissing:
			symbol = "x"
			color = colorRed
			reason = "(not linked)"
		case FileStatusDiff:
			symbol = "!"
			color = colorYellow
			reason = "(" + s.Message + ")"
		}

		displayTarget := c.formatTargetPath(s.TargetPath)
		displaySource := c.formatSourcePath(s.SourcePath)

		if reason != "" {
			fmt.Fprintf(w, "  %s%s%s %-*s  %s%s%s\n", color, symbol, colorReset, maxWidth, displayTarget, color, reason, colorReset)
		} else {
			fmt.Fprintf(w, "  %s%s%s %-*s -> %s\n", color, symbol, colorReset, maxWidth, displayTarget, displaySource)
		}
	}

	return allOk, nil
}

// PrintSummary prints a compact status using [ok]/[x] format.
// Returns true if all dotfiles are in sync.
func (c *Checker) PrintSummary(w io.Writer) (bool, error) {
	allOk, statuses, err := c.Check()
	if err != nil {
		return false, err
	}

	if len(statuses) == 0 {
		return true, nil
	}

	// Collect ok count and problem names
	okCount := 0
	var problemNames []string
	for _, s := range statuses {
		if s.Status == FileStatusLinked {
			okCount++
		} else {
			problemNames = append(problemNames, s.Name)
		}
	}

	fmt.Fprintln(w, "dotfiles:")
	if okCount > 0 {
		fmt.Fprintf(w, "  %s✓%s %d ok\n", colorGreen, colorReset, okCount)
	}
	for _, name := range problemNames {
		fmt.Fprintf(w, "  %sx%s %s\n", colorRed, colorReset, name)
	}

	return allOk, nil
}

// formatTargetPath formats the target path for display (replaces HOME with ~ only if target is HOME).
func (c *Checker) formatTargetPath(path string) string {
	home, _ := os.UserHomeDir()
	targetDir := c.cfg.TargetDir

	// Only use ~ shorthand if target_dir is actually the user's home directory
	if targetDir == home && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// formatSourcePath formats the source path for display (relative to repo root).
func (c *Checker) formatSourcePath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return rel
}

