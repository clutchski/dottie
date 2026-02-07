package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal config file (dottie.yaml, not .dottie.yaml)
	configPath := filepath.Join(tmpDir, "dottie.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Check defaults
	if cfg.SourceDir != "home" {
		t.Errorf("SourceDir = %q, want %q", cfg.SourceDir, "home")
	}
	if cfg.AddDot != true {
		t.Errorf("AddDot = %v, want true", cfg.AddDot)
	}
	if cfg.Conflict != "backup" {
		t.Errorf("Conflict = %q, want %q", cfg.Conflict, "backup")
	}
	if cfg.HooksDir != "hooks" {
		t.Errorf("HooksDir = %q, want %q", cfg.HooksDir, "hooks")
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
source_dir: dotfiles
target_dir: /tmp/home
add_dot: false
backup_dir: /tmp/backups
conflict: skip
ignore:
  - README.md
  - LICENSE
  - "*.bak"
hooks_dir: scripts
`
	configPath := filepath.Join(tmpDir, ".dottie.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.SourceDir != "dotfiles" {
		t.Errorf("SourceDir = %q, want %q", cfg.SourceDir, "dotfiles")
	}
	if cfg.TargetDir != "/tmp/home" {
		t.Errorf("TargetDir = %q, want %q", cfg.TargetDir, "/tmp/home")
	}
	if cfg.AddDot != false {
		t.Errorf("AddDot = %v, want false", cfg.AddDot)
	}
	if cfg.BackupDir != "/tmp/backups" {
		t.Errorf("BackupDir = %q, want %q", cfg.BackupDir, "/tmp/backups")
	}
	if cfg.Conflict != "skip" {
		t.Errorf("Conflict = %q, want %q", cfg.Conflict, "skip")
	}
	if len(cfg.Ignore) != 3 {
		t.Errorf("len(Ignore) = %d, want 3", len(cfg.Ignore))
	}
	if cfg.HooksDir != "scripts" {
		t.Errorf("HooksDir = %q, want %q", cfg.HooksDir, "scripts")
	}
}

func TestLoadConfig_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Should succeed with defaults when no config file exists
	// (but directory has home/ marker via other tests or IsDottieDir check)
	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.SourceDir != "home" {
		t.Errorf("SourceDir = %q, want %q", cfg.SourceDir, "home")
	}
	if cfg.AddDot != true {
		t.Errorf("AddDot = %v, want true", cfg.AddDot)
	}
}

func TestIsDottieDir(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "empty directory",
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		{
			name: "has dottie.yaml",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "dottie.yaml"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "has .dottie.yaml",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, ".dottie.yaml"), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "has home dir",
			setup: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, "home"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "has hooks dir",
			setup: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, "hooks"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "has unrelated files only",
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(t, tmpDir)
			if got := IsDottieDir(tmpDir); got != tt.want {
				t.Errorf("IsDottieDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigFile_WithFile(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "dottie.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if got := cfg.File(); got != "dottie.yaml" {
		t.Errorf("ConfigFile() = %q, want %q", got, "dottie.yaml")
	}
}

func TestConfigFile_WithDotFile(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, ".dottie.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if got := cfg.File(); got != ".dottie.yaml" {
		t.Errorf("ConfigFile() = %q, want %q", got, ".dottie.yaml")
	}
}

func TestConfigFile_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if got := cfg.File(); got != "defaults" {
		t.Errorf("ConfigFile() = %q, want %q", got, "defaults")
	}
}

func TestConfig_ShouldIgnore(t *testing.T) {
	cfg := &Config{
		Ignore: []string{"README.md", "LICENSE", "*.bak"},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"exact match", "README.md", true},
		{"exact match 2", "LICENSE", true},
		{"glob match", "test.bak", true},
		{"no match", "vimrc", false},
		{"always ignored - git", ".git", true},
		{"always ignored - config", ".dottie.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.ShouldIgnore(tt.path); got != tt.expected {
				t.Errorf("ShouldIgnore(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestConfig_GetSourcePath(t *testing.T) {
	cfg := &Config{
		SourceDir: "dotfiles",
	}
	cfg.repoRoot = "/home/user/myrepo"

	expected := "/home/user/myrepo/dotfiles"
	if got := cfg.GetSourcePath(); got != expected {
		t.Errorf("GetSourcePath() = %q, want %q", got, expected)
	}
}

func TestConfig_GetTargetPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		TargetDir: tmpDir,
		AddDot:    true,
	}

	// Test with AddDot = true
	expected := filepath.Join(tmpDir, ".vimrc")
	if got := cfg.GetTargetPath("vimrc"); got != expected {
		t.Errorf("GetTargetPath(\"vimrc\") = %q, want %q", got, expected)
	}

	// Test with AddDot = false
	cfg.AddDot = false
	expected = filepath.Join(tmpDir, "vimrc")
	if got := cfg.GetTargetPath("vimrc"); got != expected {
		t.Errorf("GetTargetPath(\"vimrc\") with AddDot=false = %q, want %q", got, expected)
	}
}
