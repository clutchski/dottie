package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileExists returns true if the path exists (file or directory).
// Note: Returns false for broken symlinks. Use PathExists to detect broken symlinks.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// PathExists returns true if anything exists at path, including broken symlinks.
func PathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// IsSymlink returns true if the path is a symbolic link.
func IsSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

// SymlinkTarget returns the target of a symbolic link.
func SymlinkTarget(path string) (string, error) {
	if !IsSymlink(path) {
		return "", fmt.Errorf("%s is not a symlink", path)
	}
	return os.Readlink(path)
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// EnsureDir creates a directory and all parent directories if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// BackupFile copies a file, symlink, or directory to the backup directory with a timestamp.
// Symlinks are preserved as symlinks. Directories are copied recursively.
// Returns the path to the backup.
func BackupFile(src, backupDir string) (string, error) {
	if err := EnsureDir(backupDir); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create backup filename with timestamp
	base := filepath.Base(src)
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.%s", base, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// If src is a symlink, preserve it as a symlink
	if IsSymlink(src) {
		target, err := os.Readlink(src)
		if err != nil {
			return "", fmt.Errorf("failed to read symlink: %w", err)
		}
		if err := os.Symlink(target, backupPath); err != nil {
			return "", fmt.Errorf("failed to backup symlink: %w", err)
		}
		return backupPath, nil
	}

	// If src is a directory, copy recursively
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("failed to stat source: %w", err)
	}
	if srcInfo.IsDir() {
		if err := CopyDir(src, backupPath); err != nil {
			return "", fmt.Errorf("failed to backup directory: %w", err)
		}
		return backupPath, nil
	}

	// Regular file - use CopyFile
	if err := CopyFile(src, backupPath); err != nil {
		return "", fmt.Errorf("failed to backup file: %w", err)
	}

	return backupPath, nil
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := srcFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// CopyDir recursively copies a directory, preserving symlinks.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Check if it's a symlink (entry.Type() includes symlink info)
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
			continue
		}

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// IsDir returns true if the path is a directory.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}
