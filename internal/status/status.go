package status

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/util"
)

// FileStatus represents the link status of a dotfile.
type FileStatus int

const (
	FileStatusLinked FileStatus = iota
	FileStatusMissing
	FileStatusConflict
)

func (s FileStatus) String() string {
	switch s {
	case FileStatusLinked:
		return "linked"
	case FileStatusMissing:
		return "missing"
	case FileStatusConflict:
		return "conflict"
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
		targetPath := c.cfg.GetTargetPath(name)

		status := c.checkFile(sourcePath, targetPath)
		status.Name = name
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

	// Check if it's a symlink
	if !util.IsSymlink(target) {
		status.Status = FileStatusConflict
		status.Message = "exists but not a symlink"
		return status
	}

	// Check if symlink points to the right place
	linkTarget, err := util.SymlinkTarget(target)
	if err != nil {
		status.Status = FileStatusConflict
		status.Message = "cannot read symlink target"
		return status
	}

	if linkTarget != source {
		status.Status = FileStatusConflict
		status.Message = fmt.Sprintf("symlink points to %s", linkTarget)
		return status
	}

	status.Status = FileStatusLinked
	status.Message = "linked"
	return status
}

// Print prints the status to stdout.
func (c *Checker) Print() error {
	statuses, err := c.GetStatus()
	if err != nil {
		return err
	}

	if len(statuses) == 0 {
		fmt.Println("No dotfiles found")
		return nil
	}

	fmt.Println("Dotfiles:")
	for _, s := range statuses {
		var indicator string
		switch s.Status {
		case FileStatusLinked:
			indicator = "[linked]  "
		case FileStatusMissing:
			indicator = "[missing] "
		case FileStatusConflict:
			indicator = "[conflict]"
		}
		fmt.Printf("  %s %s\n", indicator, s.Name)
	}

	return nil
}
