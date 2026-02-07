package update

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/clutchski/dottie/internal/util"
)

var apiURL = "https://api.github.com/repos/clutchski/dottie/releases/latest"
var downloadURL = "https://github.com/clutchski/dottie/releases/download"

var executablePath = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// fetchLatestVersion queries the GitHub API for the latest release tag.
func fetchLatestVersion() (string, error) {
	resp, err := http.Get(apiURL)
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

func downloadAndInstall(version string) error {
	osName := capitalizeOS(util.DetectOS())
	arch := util.DetectArch()
	tarballName := fmt.Sprintf("dottie_%s_%s_%s.tar.gz", version, osName, arch)
	url := fmt.Sprintf("%s/v%s/%s", downloadURL, version, tarballName)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading release: status %d", resp.StatusCode)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("decompressing release: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("dottie binary not found in tarball")
		}
		if err != nil {
			return fmt.Errorf("reading tarball: %w", err)
		}
		if filepath.Base(hdr.Name) != "dottie" {
			continue
		}

		exePath, err := executablePath()
		if err != nil {
			return fmt.Errorf("locating executable: %w", err)
		}

		dir := filepath.Dir(exePath)
		tmp, err := os.CreateTemp(dir, "dottie-update-*")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := io.Copy(tmp, tr); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing binary: %w", err)
		}
		tmp.Close()

		if err := os.Chmod(tmpPath, 0o755); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("setting permissions: %w", err)
		}

		if err := os.Rename(tmpPath, exePath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("replacing binary: %w", err)
		}

		fmt.Printf("Updated dottie to %s (%s/%s) -> %s\n", version, osName, arch, exePath)
		return nil
	}
}

// VersionStatus holds the result of an async version check.
type VersionStatus struct {
	Latest   string
	UpToDate bool
	Err      error
}

// GetVersion starts an async version check and returns a channel with the result.
func GetVersion(currentVersion string) <-chan VersionStatus {
	ch := make(chan VersionStatus, 1)
	go func() {
		latest, err := fetchLatestVersion()
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

// Run downloads and installs the latest release to update dottie.
// If currentVersion matches the latest release, it prints a message and returns.
// If currentVersion is "dev", the check is skipped and it always updates.
func Run(currentVersion string) error {
	latest := ""
	if currentVersion != "dev" {
		var err error
		latest, err = fetchLatestVersion()
		if err != nil {
			return err
		}
		if normalizeVersion(currentVersion) == normalizeVersion(latest) {
			fmt.Printf("dottie is already up to date (%s)\n", latest)
			return nil
		}
	}

	version := normalizeVersion(latest)
	if currentVersion == "dev" {
		// For dev builds, fetch the latest to know what version to download
		var err error
		latest, err = fetchLatestVersion()
		if err != nil {
			return err
		}
		version = normalizeVersion(latest)
	}

	return downloadAndInstall(version)
}
