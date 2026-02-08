package update

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clutchski/dottie/internal/util"
)

const (
	defaultAPIURL      = "https://api.github.com/repos/clutchski/dottie/releases/latest"
	defaultDownloadURL = "https://github.com/clutchski/dottie/releases/download"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// fetchLatestVersion queries the GitHub API for the latest release tag.
func fetchLatestVersion(apiURL string) (string, error) {
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching latest version: status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decoding release response: %w", err)
	}

	return release.TagName, nil
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

func capitalizeOS(os string) string {
	switch os {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	default:
		return os
	}
}

// extractBinary reads a tar.gz stream and writes the "dottie" entry to w.
func extractBinary(r io.Reader, w io.Writer) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("decompressing release: %w", err)
	}
	defer gr.Close()

	const maxBinarySize = 100 << 20 // 100 MB

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("dottie binary not found in tarball")
		}
		if err != nil {
			return fmt.Errorf("reading tarball: %w", err)
		}
		if filepath.Base(hdr.Name) != "dottie" {
			continue
		}
		_, err = io.Copy(w, io.LimitReader(tr, maxBinarySize))
		return err
	}
}

// atomicReplace sets srcPath executable and renames it over destPath.
func atomicReplace(srcPath, destPath string) error {
	if err := os.Chmod(srcPath, 0o755); err != nil {
		os.Remove(srcPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		os.Remove(srcPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// downloadAndInstall downloads a tarball from url and installs the dottie binary to targetPath.
func downloadAndInstall(url, targetPath string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading release: status %d", resp.StatusCode)
	}

	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, "dottie-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if err := extractBinary(resp.Body, tmp); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()

	return atomicReplace(tmpPath, targetPath)
}

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating executable: %w", err)
	}
	return filepath.EvalSymlinks(exe)
}

// VersionStatus holds the result of an async version check.
type VersionStatus struct {
	Latest   string
	UpToDate bool
	Err      error
}

// GetVersionFrom starts an async version check against apiURL and returns a channel with the result.
func GetVersionFrom(currentVersion, apiURL string) <-chan VersionStatus {
	ch := make(chan VersionStatus, 1)
	go func() {
		latest, err := fetchLatestVersion(apiURL)
		if err != nil {
			ch <- VersionStatus{Err: err}
			return
		}
		ch <- VersionStatus{
			Latest:   latest,
			UpToDate: normalizeVersion(currentVersion) == normalizeVersion(latest),
		}
	}()
	return ch
}

// GetVersion starts an async version check and returns a channel with the result.
func GetVersion(currentVersion string) <-chan VersionStatus {
	return GetVersionFrom(currentVersion, defaultAPIURL)
}

// InstallFrom fetches the latest version from apiURL, downloads the release from
// baseDownloadURL, and installs it to exePath. It is the test-friendly variant of Install.
func InstallFrom(currentVersion, apiURL, baseDownloadURL, exePath string) error {
	latest, err := fetchLatestVersion(apiURL)
	if err != nil {
		return err
	}

	if currentVersion != "dev" && normalizeVersion(currentVersion) == normalizeVersion(latest) {
		fmt.Printf("dottie is already up to date (%s)\n", latest)
		return nil
	}

	version := normalizeVersion(latest)
	osName := capitalizeOS(util.DetectOS())
	arch := util.DetectArch()
	tarballName := fmt.Sprintf("dottie_%s_%s_%s.tar.gz", version, osName, arch)
	downloadURL := fmt.Sprintf("%s/v%s/%s", baseDownloadURL, version, tarballName)

	if err := downloadAndInstall(downloadURL, exePath); err != nil {
		return err
	}

	fmt.Printf("Updated dottie to %s (%s/%s) -> %s\n", version, osName, arch, exePath)
	return nil
}

// Install downloads and installs the latest release to update dottie.
// If currentVersion matches the latest release, it prints a message and returns.
// If currentVersion is "dev", the check is skipped and it always updates.
func Install(currentVersion string) error {
	exePath, err := resolveExecutable()
	if err != nil {
		return err
	}
	return InstallFrom(currentVersion, defaultAPIURL, defaultDownloadURL, exePath)
}
