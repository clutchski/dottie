package util

import "runtime"

// DetectOS returns the current operating system.
func DetectOS() string {
	return runtime.GOOS
}

// DetectArch returns the current architecture.
func DetectArch() string {
	return runtime.GOARCH
}

// IsDarwin returns true if running on macOS.
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}
