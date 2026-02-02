package link

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/util"
)

// Status represents the result of a link operation.
type Status int

const (
	StatusLinked Status = iota
	StatusWouldLink
	StatusAlreadyLinked
	StatusSkipped
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusLinked:
		return "linked"
	case StatusWouldLink:
		return "would link"
	case StatusAlreadyLinked:
		return "already linked"
	case StatusSkipped:
		return "skipped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Result represents the outcome of linking a single file.
type Result struct {
	Source     string
	Target     string
	Status     Status
	BackupPath string
	Error      error
}

// Linker handles symlinking dotfiles.
type Linker struct {
	cfg *config.Config
}

// New creates a new Linker.
func New(cfg *config.Config) *Linker {
	return &Linker{cfg: cfg}
}

// Link symlinks all dotfiles from source to target.
// If dryRun is true, no changes are made.
// If force is true, existing files are overwritten without backup.
func (l *Linker) Link(dryRun, force bool) ([]Result, error) {
	sourceDir := l.cfg.GetSourcePath()

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory: %w", err)
	}

	var results []Result

	for _, entry := range entries {
		name := entry.Name()

		if l.cfg.ShouldIgnore(name) {
			continue
		}

		sourcePath := filepath.Join(sourceDir, name)
		targetPath := l.cfg.GetTargetPath(name)

		result := l.linkOne(sourcePath, targetPath, dryRun, force)
		results = append(results, result)
	}

	return results, nil
}

func (l *Linker) linkOne(source, target string, dryRun, force bool) Result {
	result := Result{
		Source: source,
		Target: target,
	}

	// Check if target already exists
	if util.FileExists(target) {
		// If it's already a symlink pointing to the right place, skip
		if util.IsSymlink(target) {
			linkTarget, err := util.SymlinkTarget(target)
			if err == nil && linkTarget == source {
				result.Status = StatusAlreadyLinked
				return result
			}
		}

		// Handle existing file
		if dryRun {
			result.Status = StatusWouldLink
			return result
		}

		if !force {
			// Backup existing file
			backupDir, err := l.cfg.GetBackupPath()
			if err != nil {
				result.Status = StatusError
				result.Error = fmt.Errorf("failed to get backup path: %w", err)
				return result
			}

			backupPath, err := util.BackupFile(target, backupDir)
			if err != nil {
				result.Status = StatusError
				result.Error = fmt.Errorf("failed to backup file: %w", err)
				return result
			}
			result.BackupPath = backupPath
		}

		// Remove existing file/symlink
		if err := os.RemoveAll(target); err != nil {
			result.Status = StatusError
			result.Error = fmt.Errorf("failed to remove existing file: %w", err)
			return result
		}
	}

	if dryRun {
		result.Status = StatusWouldLink
		return result
	}

	// Ensure parent directory exists
	if err := util.EnsureDir(filepath.Dir(target)); err != nil {
		result.Status = StatusError
		result.Error = fmt.Errorf("failed to create parent directory: %w", err)
		return result
	}

	// Create symlink
	if err := os.Symlink(source, target); err != nil {
		result.Status = StatusError
		result.Error = fmt.Errorf("failed to create symlink: %w", err)
		return result
	}

	result.Status = StatusLinked
	return result
}
