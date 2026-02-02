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

func TestBackupFile_Symlink(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a target file and a symlink to it
	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	symlink := filepath.Join(tmpDir, "link")
	if err := os.Symlink(targetFile, symlink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Backup the symlink
	backupPath, err := BackupFile(symlink, backupDir)
	if err != nil {
		t.Fatalf("BackupFile() returned error: %v", err)
	}

	// Check backup exists
	if !PathExists(backupPath) {
		t.Fatalf("Backup not created at %q", backupPath)
	}

	// Check backup is a symlink (not a regular file)
	if !IsSymlink(backupPath) {
		t.Errorf("Backup at %q is not a symlink", backupPath)
	}

	// Check symlink points to the same target
	backupTarget, err := os.Readlink(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup symlink: %v", err)
	}
	if backupTarget != targetFile {
		t.Errorf("Backup symlink target = %q, want %q", backupTarget, targetFile)
	}
}

func TestBackupFile_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a directory with files and a symlink inside
	sourceDir := filepath.Join(tmpDir, "mydir")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}

	// Create a regular file inside the directory
	fileInDir := filepath.Join(sourceDir, "file.txt")
	if err := os.WriteFile(fileInDir, []byte("file content"), 0644); err != nil {
		t.Fatalf("Failed to create file in dir: %v", err)
	}

	// Create a symlink inside the directory
	linkTarget := filepath.Join(tmpDir, "external.txt")
	if err := os.WriteFile(linkTarget, []byte("external"), 0644); err != nil {
		t.Fatalf("Failed to create external file: %v", err)
	}
	linkInDir := filepath.Join(sourceDir, "link")
	if err := os.Symlink(linkTarget, linkInDir); err != nil {
		t.Fatalf("Failed to create symlink in dir: %v", err)
	}

	// Backup the directory
	backupPath, err := BackupFile(sourceDir, backupDir)
	if err != nil {
		t.Fatalf("BackupFile() returned error: %v", err)
	}

	// Check backup is a directory
	if !IsDir(backupPath) {
		t.Fatalf("Backup at %q is not a directory", backupPath)
	}

	// Check file inside backup
	backupFile := filepath.Join(backupPath, "file.txt")
	content, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("Failed to read backed up file: %v", err)
	}
	if string(content) != "file content" {
		t.Errorf("Backed up file content = %q, want %q", content, "file content")
	}

	// Check symlink inside backup is preserved as a symlink
	backupLink := filepath.Join(backupPath, "link")
	if !IsSymlink(backupLink) {
		t.Errorf("Backed up link at %q is not a symlink", backupLink)
	}
	backupLinkTarget, err := os.Readlink(backupLink)
	if err != nil {
		t.Fatalf("Failed to read backed up symlink: %v", err)
	}
	if backupLinkTarget != linkTarget {
		t.Errorf("Backed up symlink target = %q, want %q", backupLinkTarget, linkTarget)
	}
}
