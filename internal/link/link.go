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

// FileInfo represents the status of a single dotfile.
type FileInfo struct {
	Name       string
	SourcePath string
	TargetPath string
	Status     FileStatus
	Message    string
}

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

// Event represents the outcome of linking a single file.
type Event struct {
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

// Run symlinks all dotfiles from source to target, streaming results as they happen.
// If dryRun is true, no changes are made.
// If force is true, existing files are overwritten without backup.
func (l *linker) Run(dryRun, force bool) (<-chan Event, error) {
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

	ch := make(chan Event, len(sources))
	go func() {
		defer close(ch)
		sourceDir := l.cfg.GetSourcePath()
		for _, source := range sources {
			relPath, _ := filepath.Rel(sourceDir, source)
			target := l.computeTargetPath(relPath)
			ch <- l.linkFile(source, target, dryRun, force)
		}
	}()

	return ch, nil
}

// Link symlinks all dotfiles, returning collected results.
func (l *linker) Link(dryRun, force bool) ([]Event, error) {
	ch, err := l.Run(dryRun, force)
	if err != nil {
		return nil, err
	}
	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events, nil
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

func (l *linker) linkFile(source, target string, dryRun, force bool) Event {
	result := Event{
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

// CheckStatus returns the status of all dotfiles.
func (l *linker) CheckStatus() ([]FileInfo, error) {
	sourceDir := l.cfg.GetSourcePath()
	return l.checkStatusDir(sourceDir, l.cfg.TargetDir, "", true)
}

// checkStatusDir recursively gets status of files in sourceDir.
// prefix is used to build display names like "config/starship.toml".
func (l *linker) checkStatusDir(sourceDir, targetDir, prefix string, topLevel bool) ([]FileInfo, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory: %w", err)
	}

	var statuses []FileInfo

	for _, entry := range entries {
		name := entry.Name()

		if l.cfg.ShouldIgnore(name) {
			continue
		}

		sourcePath := filepath.Join(sourceDir, name)

		var targetPath string
		var displayName string
		if topLevel {
			targetPath = l.cfg.GetTargetPath(name)
			displayName = name
		} else {
			targetPath = filepath.Join(targetDir, name)
			displayName = filepath.Join(prefix, name)
		}

		if entry.IsDir() {
			subStatuses, err := l.checkStatusDir(sourcePath, targetPath, displayName, false)
			if err != nil {
				return statuses, err
			}
			statuses = append(statuses, subStatuses...)
			continue
		}

		info := l.checkFileStatus(sourcePath, targetPath)
		info.Name = displayName
		info.SourcePath = sourcePath
		info.TargetPath = targetPath

		statuses = append(statuses, info)
	}

	return statuses, nil
}

func (l *linker) checkFileStatus(source, target string) FileInfo {
	info := FileInfo{}

	if !util.FileExists(target) {
		info.Status = FileStatusMissing
		info.Message = "not linked"
		return info
	}

	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		info.Status = FileStatusDiff
		info.Message = "cannot resolve target path"
		return info
	}

	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		resolvedSource = source
	}

	if resolvedTarget == resolvedSource {
		info.Status = FileStatusLinked
		info.Message = "linked"
		return info
	}

	if util.IsSymlink(target) {
		linkTarget, _ := util.SymlinkTarget(target)
		info.Status = FileStatusDiff
		info.Message = fmt.Sprintf("symlink points to %s", linkTarget)
	} else {
		info.Status = FileStatusDiff
		info.Message = "exists but not linked to repo"
	}
	return info
}
