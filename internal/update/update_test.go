package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := tw.WriteHeader(&tar.Header{
		Name: filename,
		Size: int64(len(content)),
		Mode: 0o755,
	})
	require.NoError(t, err)

	_, err = tw.Write(content)
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestDownloadAndInstall(t *testing.T) {
	tarball := createTarGz(t, "dottie", []byte("fake-binary"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()

	origDownloadURL := downloadURL
	downloadURL = srv.URL
	defer func() { downloadURL = origDownloadURL }()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	origExecPath := executablePath
	executablePath = func() (string, error) { return target, nil }
	defer func() { executablePath = origExecPath }()

	err := downloadAndInstall("1.2.3")
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "fake-binary", string(got))

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestFetchLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	got, err := fetchLatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", got)
}

func TestRunSkipsWhenUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	// Should return nil without running install (passing matching version)
	err := Run("v1.2.3")
	assert.NoError(t, err)
}

func TestRunSkipsWhenUpToDateWithoutPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	// Should match even without the v prefix
	err := Run("1.2.3")
	assert.NoError(t, err)
}

func TestRunProceedsWhenOutdated(t *testing.T) {
	tarball := createTarGz(t, "dottie", []byte("new-binary"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintln(w, `{"tag_name": "v2.0.0"}`)
			return
		}
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	origDownloadURL := downloadURL
	downloadURL = srv.URL
	defer func() { downloadURL = origDownloadURL }()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	origExecPath := executablePath
	executablePath = func() (string, error) { return target, nil }
	defer func() { executablePath = origExecPath }()

	err := Run("v1.0.0")
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(got))
}

func TestRunSkipsCheckForDevVersion(t *testing.T) {
	tarball := createTarGz(t, "dottie", []byte("dev-binary"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintln(w, `{"tag_name": "v9.9.9"}`)
			return
		}
		_, _ = w.Write(tarball)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	origDownloadURL := downloadURL
	downloadURL = srv.URL
	defer func() { downloadURL = origDownloadURL }()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	origExecPath := executablePath
	executablePath = func() (string, error) { return target, nil }
	defer func() { executablePath = origExecPath }()

	err := Run("dev")
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "dev-binary", string(got))
}

func TestGetVersionUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	result := <-GetVersion("v1.2.3")
	require.NoError(t, result.Err)
	assert.Equal(t, "v1.2.3", result.Latest)
	assert.True(t, result.UpToDate)
}

func TestGetVersionOutdated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v2.0.0"}`)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	result := <-GetVersion("v1.0.0")
	require.NoError(t, result.Err)
	assert.Equal(t, "v2.0.0", result.Latest)
	assert.False(t, result.UpToDate)
}
