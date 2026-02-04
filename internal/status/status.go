package status

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/util"
)

// defaultPatterns are known dotfile patterns to scan for in HOME.
var defaultPatterns = []string{
	// Shell configs
	".bashrc", ".bash_profile", ".zshrc", ".zshenv", ".zprofile", ".profile",

	// Editors
	".vimrc", ".vim/", ".emacs", ".emacs.d/",

	// Git
	".gitconfig", ".gitignore_global",

	// Terminal
	".tmux.conf", ".inputrc",

	// XDG config - scan all immediate children
	".config/*",
}

// FileStatus represents the link status of a dotfile.
type FileStatus int

const (
	FileStatusLinked FileStatus = iota
	FileStatusMissing
	FileStatusDiff
	FileStatusUntracked
)

func (s FileStatus) String() string {
	switch s {
	case FileStatusLinked:
		return "linked"
	case FileStatusMissing:
		return "unlinked"
	case FileStatusDiff:
		return "diff"
	case FileStatusUntracked:
		return "untracked"
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

// GetStatusScan scans HOME for known dotfile patterns and returns their status.
func (c *Checker) GetStatusScan() ([]DotfileStatus, error) {
	home := c.cfg.TargetDir
	sourceDir := c.cfg.GetSourcePath()

	var statuses []DotfileStatus
	seen := make(map[string]bool)

	// Scan known patterns in HOME
	for _, pattern := range defaultPatterns {
		if strings.HasSuffix(pattern, "/*") {
			// Expand wildcard pattern (e.g., .config/*)
			dir := strings.TrimSuffix(pattern, "/*")
			dirPath := filepath.Join(home, dir)

			// Skip if the parent directory is already a symlink to repo
			// (its contents are already covered by the parent)
			if util.IsSymlink(dirPath) {
				continue
			}

			entries, err := os.ReadDir(dirPath)
			if err != nil {
				continue // directory doesn't exist, skip
			}
			for _, entry := range entries {
				targetPath := filepath.Join(dirPath, entry.Name())
				status := c.checkTargetFile(targetPath, sourceDir, home)
				if status != nil && !seen[status.TargetPath] {
					seen[status.TargetPath] = true
					statuses = append(statuses, *status)
				}
			}
		} else {
			// Direct pattern
			targetPath := filepath.Join(home, pattern)
			status := c.checkTargetFile(targetPath, sourceDir, home)
			if status != nil && !seen[status.TargetPath] {
				seen[status.TargetPath] = true
				statuses = append(statuses, *status)
			}
		}
	}

	// Also check repo files that might not match patterns (for "missing" status)
	repoStatuses, err := c.GetStatus()
	if err != nil {
		return nil, err
	}
	for _, rs := range repoStatuses {
		if !seen[rs.TargetPath] {
			seen[rs.TargetPath] = true
			statuses = append(statuses, rs)
		}
	}

	// Sort by status (ok, diff, missing, untracked) then by path
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Status != statuses[j].Status {
			return statuses[i].Status < statuses[j].Status
		}
		return statuses[i].TargetPath < statuses[j].TargetPath
	})

	return statuses, nil
}

// checkTargetFile checks a file in the target directory and returns its status.
// Directories are skipped - only files are reported.
func (c *Checker) checkTargetFile(targetPath, sourceDir, home string) *DotfileStatus {
	if !util.FileExists(targetPath) {
		return nil // doesn't exist
	}

	// Skip directories - only report files
	if util.IsDir(targetPath) && !util.IsSymlink(targetPath) {
		return nil
	}

	// Calculate the expected source path
	relPath, err := filepath.Rel(home, targetPath)
	if err != nil {
		return nil
	}

	// Convert target name to source name (remove leading dots if add_dot is true)
	// e.g., .vimrc -> vimrc, .config/starship.toml -> config/starship.toml
	sourceName := relPath
	if c.cfg.AddDot {
		parts := strings.Split(relPath, string(filepath.Separator))
		for i, part := range parts {
			if strings.HasPrefix(part, ".") {
				parts[i] = strings.TrimPrefix(part, ".")
			}
		}
		sourceName = filepath.Join(parts...)
	}

	sourcePath := filepath.Join(sourceDir, sourceName)

	status := &DotfileStatus{
		Name:       relPath,
		SourcePath: sourcePath,
		TargetPath: targetPath,
	}

	// Check if source exists in repo
	if !util.FileExists(sourcePath) {
		status.Status = FileStatusUntracked
		status.Message = "not in repo"
		return status
	}

	// Check if it's properly linked
	if !util.IsSymlink(targetPath) {
		status.Status = FileStatusDiff
		status.Message = "exists but not a symlink"
		return status
	}

	linkTarget, err := util.SymlinkTarget(targetPath)
	if err != nil {
		status.Status = FileStatusDiff
		status.Message = "cannot read symlink target"
		return status
	}

	if linkTarget != sourcePath {
		status.Status = FileStatusDiff
		status.Message = fmt.Sprintf("symlink points to %s", linkTarget)
		return status
	}

	status.Status = FileStatusLinked
	status.Message = "linked"
	return status
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
)

// Check returns true if all dotfiles are in sync, false otherwise.
// Also returns the list of statuses for display.
func (c *Checker) Check() (bool, []DotfileStatus, error) {
	statuses, err := c.GetStatusScan()
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

// Print prints the status to stdout.
// Returns true if all dotfiles are in sync.
func (c *Checker) Print() (bool, error) {
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

	fmt.Println("Dotfiles:")
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
		case FileStatusUntracked:
			symbol = "?"
			color = colorGray
			reason = "(not in repo)"
		}

		displayTarget := c.formatTargetPath(s.TargetPath)
		displaySource := c.formatSourcePath(s.SourcePath)

		if reason != "" {
			fmt.Printf("  %s[%s]%s %-*s  %s%s%s\n", color, symbol, colorReset, maxWidth, displayTarget, color, reason, colorReset)
		} else {
			fmt.Printf("  %s[%s]%s %-*s -> %s\n", color, symbol, colorReset, maxWidth, displayTarget, displaySource)
		}
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

