package link

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

// linker handles symlinking dotfiles.
type linker struct {
	cfg *config.Config
}

// New creates a new linker.
func New(cfg *config.Config) *linker {
	return &linker{cfg: cfg}

}

// Link symlinks all dotfiles from source to target.
// If dryRun is true, no changes are made.
// If force is true, existing files are overwritten without backup.
func (l *linker) Link(dryRun, force bool) ([]Result, error) {
	sources, err := l.collectSourcePaths()
	if err != nil {
		return nil, err
	}

	if !dryRun {
		dirs := l.getTargetDirs(sources)
		if err := l.makeDirs(dirs); err != nil {
			return nil, err
		}
	}

	return l.createLinks(sources, dryRun, force), nil
}

// collectSourcePaths walks the source directory and collects all file paths.
func (l *linker) collectSourcePaths() ([]string, error) {
	sourceDir := l.cfg.GetSourcePath()
	var sources []string

	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == sourceDir {
			return nil
		}

		if l.cfg.ShouldIgnore(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() {
			sources = append(sources, path)
		}
		return nil
	})

	return sources, err
}

// getTargetDirs extracts unique target directories from source paths.
func (l *linker) getTargetDirs(sourcePaths []string) []string {
	seen := make(map[string]bool)
	var dirs []string

	sourceDir := l.cfg.GetSourcePath()

	for _, source := range sourcePaths {
		relPath, err := filepath.Rel(sourceDir, source)
		if err != nil {
			continue
		}

		parts := strings.Split(filepath.Dir(relPath), string(filepath.Separator))
		if len(parts) == 1 && parts[0] == "." {
			continue
		}

		currentTarget := l.cfg.TargetDir
		for i, part := range parts {
			targetPart := part
			if i == 0 && l.cfg.AddDot && !strings.HasPrefix(part, ".") {
				targetPart = "." + part
			}
			currentTarget = filepath.Join(currentTarget, targetPart)

			if !seen[currentTarget] {
				seen[currentTarget] = true
				dirs = append(dirs, currentTarget)
			}
		}
	}
	return dirs
}

// makeDirs creates directories, replacing any symlinks with real directories.
func (l *linker) makeDirs(dirs []string) error {
	for _, dir := range dirs {
		if util.IsSymlink(dir) {
			if err := os.Remove(dir); err != nil {
				return fmt.Errorf("failed to remove symlink %s: %w", dir, err)
			}
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// createLinks creates symlinks for all source files.
func (l *linker) createLinks(sources []string, dryRun, force bool) []Result {
	sourceDir := l.cfg.GetSourcePath()
	var results []Result

	for _, source := range sources {
		relPath, _ := filepath.Rel(sourceDir, source)
		target := l.computeTargetPath(relPath)
		results = append(results, l.linkFile(source, target, dryRun, force))
	}
	return results
}

// computeTargetPath computes the target path for a file given its relative path.
func (l *linker) computeTargetPath(relPath string) string {
	parts := strings.Split(relPath, string(filepath.Separator))

	// First component gets dot prefix via existing config method
	firstTarget := l.cfg.GetTargetPath(parts[0])

	if len(parts) == 1 {
		return firstTarget
	}
	return filepath.Join(firstTarget, filepath.Join(parts[1:]...))
}

func (l *linker) linkFile(source, target string, dryRun, force bool) Result {
	result := Result{
		Source: source,
		Target: target,
	}

	// Check if target already exists (including broken symlinks)
	if util.PathExists(target) {
		// If it's already a symlink pointing to the right place, skip
		if util.IsSymlink(target) {
			linkTarget, err := util.SymlinkTarget(target)
			if err == nil && linkTarget == source {
				result.Status = StatusAlreadyLinked
				return result
			}
		}

		// Check if target resolves to the same file as source
		// (e.g., through a parent directory symlink like ~/.config -> dotfiles/config)
		targetInfo, targetErr := os.Stat(target)
		sourceInfo, sourceErr := os.Stat(source)
		if targetErr == nil && sourceErr == nil && os.SameFile(targetInfo, sourceInfo) {
			result.Status = StatusAlreadyLinked
			return result
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

	// Create symlink (directories already prepared by prepareDirectories)
	if err := os.Symlink(source, target); err != nil {
		result.Status = StatusError
		result.Error = fmt.Errorf("failed to create symlink: %w", err)
		return result
	}

	result.Status = StatusLinked
	return result
}
