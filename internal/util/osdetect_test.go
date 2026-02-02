package util

import (
	"runtime"
	"testing"
)

func TestDetectOS(t *testing.T) {
	os := DetectOS()
	if os != runtime.GOOS {
		t.Errorf("DetectOS() = %q, want %q", os, runtime.GOOS)
	}
}

func TestDetectArch(t *testing.T) {
	arch := DetectArch()
	if arch != runtime.GOARCH {
		t.Errorf("DetectArch() = %q, want %q", arch, runtime.GOARCH)
	}
}

func TestIsDarwin(t *testing.T) {
	isDarwin := IsDarwin()
	expected := runtime.GOOS == "darwin"
	if isDarwin != expected {
		t.Errorf("IsDarwin() = %v, want %v", isDarwin, expected)
	}
}

func TestIsLinux(t *testing.T) {
	isLinux := IsLinux()
	expected := runtime.GOOS == "linux"
	if isLinux != expected {
		t.Errorf("IsLinux() = %v, want %v", isLinux, expected)
	}
}
