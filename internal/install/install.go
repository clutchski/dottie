package install

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Installer handles package installation.
type Installer struct {
	depsDir string
}

// New creates a new Installer.
func New(depsDir string) *Installer {
	return &Installer{depsDir: depsDir}
}

// Install installs packages for the given OS.
func (i *Installer) Install(osName string, dryRun bool) error {
	packages, err := i.ListPackages(osName)
	if err != nil {
		return err
	}

	if len(packages) == 0 {
		fmt.Println("No packages to install")
		return nil
	}

	switch osName {
	case "darwin":
		return i.installBrew(dryRun)
	case "linux":
		return i.installApt(packages, dryRun)
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}
}

// ListPackages returns the list of packages for the given OS.
func (i *Installer) ListPackages(osName string) ([]string, error) {
	depsFile := i.GetDepsFile(osName)

	switch osName {
	case "darwin":
		return ParseBrewfile(depsFile)
	case "linux":
		return ParseAptFile(depsFile)
	default:
		return nil, fmt.Errorf("unsupported OS: %s", osName)
	}
}

// GetDepsFile returns the path to the deps file for the given OS.
func (i *Installer) GetDepsFile(osName string) string {
	switch osName {
	case "darwin":
		return filepath.Join(i.depsDir, "Brewfile")
	case "linux":
		return filepath.Join(i.depsDir, "apt.txt")
	default:
		return ""
	}
}

func (i *Installer) installBrew(dryRun bool) error {
	brewfile := i.GetDepsFile("darwin")

	if _, err := os.Stat(brewfile); os.IsNotExist(err) {
		fmt.Println("No Brewfile found")
		return nil
	}

	if dryRun {
		fmt.Printf("[dry-run] would run: brew bundle --file=%s\n", brewfile)
		return nil
	}

	// Check if all packages are already installed (fast)
	checkCmd := exec.Command("brew", "bundle", "check", "--file="+brewfile)
	if err := checkCmd.Run(); err == nil {
		fmt.Println("All Brewfile packages already installed")
		return nil
	}

	// Something is missing, run full install
	fmt.Println("Installing missing packages from Brewfile...")
	cmd := exec.Command("brew", "bundle", "--file="+brewfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew bundle failed: %w", err)
	}

	return nil
}

func (i *Installer) installApt(packages []string, dryRun bool) error {
	if len(packages) == 0 {
		fmt.Println("No apt packages to install")
		return nil
	}

	args := append([]string{"install", "-y"}, packages...)

	if dryRun {
		fmt.Printf("[dry-run] would run: sudo apt-get %s\n", strings.Join(args, " "))
		return nil
	}

	cmd := exec.Command("sudo", append([]string{"apt-get"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install failed: %w", err)
	}

	return nil
}

// ParseBrewfile parses a Brewfile and returns the list of brew packages.
// Only extracts `brew "package"` lines, ignoring cask, tap, etc.
func ParseBrewfile(path string) (packages []string, err error) {
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Match: brew "package" or brew 'package'
	brewRegex := regexp.MustCompile(`^brew\s+["']([^"']+)["']`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if matches := brewRegex.FindStringSubmatch(line); matches != nil {
			packages = append(packages, matches[1])
		}
	}

	return packages, scanner.Err()
}

// PrintStatus prints the status of brew packages.
func (i *Installer) PrintStatus() error {
	if runtime.GOOS != "darwin" {
		return nil // Only check brew on macOS
	}

	brewfile := i.GetDepsFile("darwin")

	if _, err := os.Stat(brewfile); os.IsNotExist(err) {
		return nil // No Brewfile, nothing to check
	}

	fmt.Println()
	fmt.Println("Brewfile:")

	cmd := exec.Command("brew", "bundle", "check", "--file="+brewfile, "--verbose")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// brew bundle check exits non-zero when packages are missing
		fmt.Print(string(output))
		return nil
	}

	fmt.Println("  All packages installed")
	return nil
}

// ParseAptFile parses an apt.txt file and returns the list of packages.
// Each line is a package name. Lines starting with # are comments.
func ParseAptFile(path string) (packages []string, err error) {
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		packages = append(packages, line)
	}

	return packages, scanner.Err()
}
