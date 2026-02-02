package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"existing file", tmpFile, true},
		{"non-existing file", filepath.Join(tmpDir, "nonexistent"), false},
		{"existing directory", tmpDir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileExists(tt.path); got != tt.expected {
				t.Errorf("FileExists(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink
	symlink := filepath.Join(tmpDir, "symlink")
	if err := os.Symlink(regularFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"symlink", symlink, true},
		{"regular file", regularFile, false},
		{"non-existing", filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSymlink(tt.path); got != tt.expected {
				t.Errorf("IsSymlink(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestSymlinkTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a symlink
	symlink := filepath.Join(tmpDir, "symlink")
	if err := os.Symlink(regularFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	target, err := SymlinkTarget(symlink)
	if err != nil {
		t.Errorf("SymlinkTarget(%q) returned error: %v", symlink, err)
	}
	if target != regularFile {
		t.Errorf("SymlinkTarget(%q) = %q, want %q", symlink, target, regularFile)
	}

	// Test non-symlink
	_, err = SymlinkTarget(regularFile)
	if err == nil {
		t.Error("SymlinkTarget on regular file should return error")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"tilde only", "~", home},
		{"tilde with path", "~/dotfiles", filepath.Join(home, "dotfiles")},
		{"absolute path", "/tmp/test", "/tmp/test"},
		{"relative path", "relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandPath(tt.path)
			if err != nil {
				t.Errorf("ExpandPath(%q) returned error: %v", tt.path, err)
			}
			if got != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "new", "nested", "dir")

	err := EnsureDir(newDir)
	if err != nil {
		t.Errorf("EnsureDir(%q) returned error: %v", newDir, err)
	}

	if !FileExists(newDir) {
		t.Errorf("EnsureDir(%q) did not create directory", newDir)
	}

	// Call again - should not error
	err = EnsureDir(newDir)
	if err != nil {
		t.Errorf("EnsureDir(%q) on existing dir returned error: %v", newDir, err)
	}
}

func TestBackupFile(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a file to backup
	originalFile := filepath.Join(tmpDir, "original.txt")
	content := []byte("original content")
	if err := os.WriteFile(originalFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	backupPath, err := BackupFile(originalFile, backupDir)
	if err != nil {
		t.Errorf("BackupFile() returned error: %v", err)
	}

	// Check backup exists
	if !FileExists(backupPath) {
		t.Errorf("Backup file not created at %q", backupPath)
	}

	// Check backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Errorf("Failed to read backup file: %v", err)
	}
	if string(backupContent) != string(content) {
		t.Errorf("Backup content = %q, want %q", backupContent, content)
	}
}
