package link

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/util"
)

// Status represents the result of a link operation or check.
type Status int

const (
	StatusLinked Status = iota
	StatusWouldLink
	StatusAlreadyLinked
	StatusError
	StatusMissing  // target doesn't exist
	StatusDiff     // target exists but points elsewhere
	StatusDangling // symlink target no longer exists
)

func (s Status) String() string {
	switch s {
	case StatusLinked:
		return "linked"
	case StatusWouldLink:
		return "would link"
	case StatusAlreadyLinked:
		return "already linked"
	case StatusError:
		return "error"
	case StatusMissing:
		return "missing"
	case StatusDiff:
		return "diff"
	case StatusDangling:
		return "dangling"
	default:
		return "unknown"
	}
}

// Result represents the outcome of linking or checking a single file.
type Result struct {
	Source     string
	Target     string
	Name       string // display name (relative path from source dir)
	Status     Status
	BackupPath string
	Message    string // human-readable reason (for failures/diffs)
	Error      error
}

// Summary holds link counts for a run summary.
type Summary struct {
	Existing int // already linked, no change
	Added    int // newly linked
	Pruned   int // orphan/pruned
	Errors   int // link errors
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

	results := l.createLinks(sources, dryRun, force)

	if !dryRun {
		if err := l.recordManifest(results); err != nil {
			return results, fmt.Errorf("failed to update manifest: %w", err)
		}
	}

	return results, nil
}

// manifestPath returns the path to the manifest file.
func (l *Linker) manifestPath() string {
	return filepath.Join(l.cfg.TargetDir, ".dottie.links")
}

// recordManifest updates the manifest with targets from the given results.
func (l *Linker) recordManifest(results []Result) error {
	mp := l.manifestPath()
	existing, err := loadManifest(mp)
	if err != nil {
		return err
	}

	seen := make(map[string]bool, len(existing))
	for _, p := range existing {
		seen[p] = true
	}

	for _, r := range results {
		if (r.Status == StatusLinked || r.Status == StatusAlreadyLinked) && !seen[r.Target] {
			seen[r.Target] = true
			existing = append(existing, r.Target)
		}
	}

	sort.Strings(existing)
	return saveManifest(mp, existing)
}

// Check performs a read-only status check of all dotfiles.
func (l *Linker) Check() ([]Result, error) {
	sources, err := l.collectSourcePaths()
	if err != nil {
		return nil, err
	}

	sourceDir := l.cfg.GetSourcePath()
	var results []Result
	for _, source := range sources {
		relPath, err := filepath.Rel(sourceDir, source)
		if err != nil {
			results = append(results, Result{
				Source:  source,
				Status:  StatusError,
				Error:   fmt.Errorf("failed to compute relative path: %w", err),
				Message: err.Error(),
			})
			continue
		}
		target := l.computeTargetPath(relPath)
		r := l.checkFile(source, target)
		r.Name = relPath
		r.Source = source
		r.Target = target
		results = append(results, r)
	}
	return results, nil
}

func (l *Linker) checkFile(source, target string) Result {
	var r Result

	// Check if target exists
	if !util.FileExists(target) {
		r.Status = StatusMissing
		r.Message = "not linked"
		return r
	}

	// Check if target resolves to source (handles both direct symlinks
	// and files accessed through symlinked parent directories)
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		r.Status = StatusDiff
		r.Message = "cannot resolve target path"
		return r
	}

	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		resolvedSource = source
	}

	if resolvedTarget == resolvedSource {
		r.Status = StatusLinked
		return r
	}

	// Not linked - check if it's a symlink pointing elsewhere or a regular file
	if util.IsSymlink(target) {
		linkTarget, err := util.SymlinkTarget(target)
		if err != nil {
			r.Status = StatusError
			r.Message = fmt.Sprintf("cannot read symlink: %v", err)
			return r
		}
		r.Status = StatusDiff
		r.Message = fmt.Sprintf("symlink points to %s", linkTarget)
	} else {
		r.Status = StatusDiff
		r.Message = "exists but not linked to repo"
	}
	return r
}

// FindDangling reads the manifest and returns entries whose symlinks are
// dangling or already removed. It is read-only.
func (l *Linker) FindDangling() ([]Result, error) {
	targets, err := loadManifest(l.manifestPath())
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, target := range targets {
		// If the symlink itself is gone, or it's dangling, it's a prune candidate.
		_, lstatErr := os.Lstat(target)
		if lstatErr != nil {
			// Symlink was removed entirely
			relPath, relErr := filepath.Rel(l.cfg.TargetDir, target)
			if relErr != nil {
				relPath = target
			}
			results = append(results, Result{
				Target: target,
				Name:   relPath,
				Status: StatusDangling,
			})
			continue
		}

		// Symlink exists -- check if it's dangling
		if _, err := os.Stat(target); err != nil {
			linkTarget, readErr := os.Readlink(target)
			if readErr != nil {
				linkTarget = ""
			}
			relPath, relErr := filepath.Rel(l.cfg.TargetDir, target)
			if relErr != nil {
				relPath = target
			}
			results = append(results, Result{
				Source: linkTarget,
				Target: target,
				Name:   relPath,
				Status: StatusDangling,
			})
		}
	}

	return results, nil
}

// Prune finds dangling symlinks, removes them, and updates the manifest.
// Results that could not be removed will have their Error field set.
func (l *Linker) Prune() ([]Result, error) {
	dangling, err := l.FindDangling()
	if err != nil || len(dangling) == 0 {
		return dangling, err
	}

	removed := make(map[string]bool)
	for i, d := range dangling {
		if err := os.Remove(d.Target); err != nil && !os.IsNotExist(err) {
			dangling[i].Error = err
			continue
		}
		removed[d.Target] = true
	}

	if len(removed) > 0 {
		mp := l.manifestPath()
		manifest, err := loadManifest(mp)
		if err != nil {
			return dangling, err
		}
		var kept []string
		for _, entry := range manifest {
			if !removed[entry] {
				kept = append(kept, entry)
			}
		}
		if err := saveManifest(mp, kept); err != nil {
			return dangling, err
		}
	}

	return dangling, nil
}

// collectSourcePaths walks the source directory and collects all file paths.
func (l *Linker) collectSourcePaths() ([]string, error) {
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
func (l *Linker) getTargetDirs(sourcePaths []string) []string {
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
func (l *Linker) makeDirs(dirs []string) error {
	for _, dir := range dirs {
		if util.IsSymlink(dir) {
			if err := os.Remove(dir); err != nil {
				return fmt.Errorf("failed to remove symlink %s: %w", dir, err)
			}
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// createLinks creates symlinks for all source files.
func (l *Linker) createLinks(sources []string, dryRun, force bool) []Result {
	sourceDir := l.cfg.GetSourcePath()
	var results []Result

	for _, source := range sources {
		relPath, err := filepath.Rel(sourceDir, source)
		if err != nil {
			results = append(results, Result{
				Source:  source,
				Status:  StatusError,
				Error:   fmt.Errorf("failed to compute relative path: %w", err),
				Message: err.Error(),
			})
			continue
		}
		target := l.computeTargetPath(relPath)
		r := l.linkFile(source, target, dryRun, force)
		r.Name = relPath
		results = append(results, r)
	}
	return results
}

// computeTargetPath computes the target path for a file given its relative path.
func (l *Linker) computeTargetPath(relPath string) string {
	parts := strings.Split(relPath, string(filepath.Separator))

	// First component gets dot prefix via existing config method
	firstTarget := l.cfg.GetTargetPath(parts[0])

	if len(parts) == 1 {
		return firstTarget
	}
	return filepath.Join(firstTarget, filepath.Join(parts[1:]...))
}

func (l *Linker) linkFile(source, target string, dryRun, force bool) Result {
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
