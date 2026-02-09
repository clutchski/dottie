package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestExtractBinary(t *testing.T) {
	tarball := createTarGz(t, "dottie", []byte("the-binary"))

	var buf bytes.Buffer
	err := extractBinary(bytes.NewReader(tarball), &buf)
	require.NoError(t, err)
	assert.Equal(t, "the-binary", buf.String())
}

func TestExtractBinaryMissing(t *testing.T) {
	tarball := createTarGz(t, "not-dottie", []byte("wrong"))

	var buf bytes.Buffer
	err := extractBinary(bytes.NewReader(tarball), &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dottie binary not found")
}

func TestAtomicReplace(t *testing.T) {
	old := codesignFunc
	codesignFunc = func(path string) error { return nil }
	defer func() { codesignFunc = old }()

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "new")
	require.NoError(t, os.WriteFile(src, []byte("new-binary"), 0o644))

	dest := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(dest, []byte("old"), 0o755))

	err := atomicReplace(src, dest)
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(got))

	info, err := os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())

	// Source should no longer exist (it was renamed)
	assert.False(t, fileExists(src))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDownloadAndInstall(t *testing.T) {
	old := codesignFunc
	codesignFunc = func(path string) error { return nil }
	defer func() { codesignFunc = old }()

	tarball := createTarGz(t, "dottie", []byte("fake-binary"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	err := downloadAndInstall(srv.URL+"/tarball", target)
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

	got, err := fetchLatestVersion(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", got)
}

func TestInstallFromSkipsWhenUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	err := InstallFrom("v1.2.3", srv.URL, srv.URL, "/unused")
	assert.NoError(t, err)
}

func TestInstallFromSkipsWhenUpToDateWithoutPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	err := InstallFrom("1.2.3", srv.URL, srv.URL, "/unused")
	assert.NoError(t, err)
}

func TestInstallFromProceedsWhenOutdated(t *testing.T) {
	old := codesignFunc
	codesignFunc = func(path string) error { return nil }
	defer func() { codesignFunc = old }()

	tarball := createTarGz(t, "dottie", []byte("new-binary"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintln(w, `{"tag_name": "v2.0.0"}`)
			return
		}
		w.Write(tarball)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	err := InstallFrom("v1.0.0", srv.URL, srv.URL, target)
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new-binary", string(got))
}

func TestInstallFromSkipsCheckForDevVersion(t *testing.T) {
	old := codesignFunc
	codesignFunc = func(path string) error { return nil }
	defer func() { codesignFunc = old }()

	tarball := createTarGz(t, "dottie", []byte("dev-binary"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintln(w, `{"tag_name": "v9.9.9"}`)
			return
		}
		w.Write(tarball)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))

	err := InstallFrom("dev", srv.URL, srv.URL, target)
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "dev-binary", string(got))
}

func TestGetVersionFromUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v1.2.3"}`)
	}))
	defer srv.Close()

	result := <-GetVersionFrom("v1.2.3", srv.URL)
	require.NoError(t, result.Err)
	assert.Equal(t, "v1.2.3", result.Latest)
	assert.True(t, result.UpToDate)
}

func TestGetVersionFromOutdated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name": "v2.0.0"}`)
	}))
	defer srv.Close()

	result := <-GetVersionFrom("v1.0.0", srv.URL)
	require.NoError(t, result.Err)
	assert.Equal(t, "v2.0.0", result.Latest)
	assert.False(t, result.UpToDate)
}

func TestCodesignBinary(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign only available on macOS")
	}

	// Build a real Mach-O binary to sign.
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0o644))

	bin := filepath.Join(tmpDir, "testbin")
	cmd := exec.Command("go", "build", "-o", bin, src)
	require.NoError(t, cmd.Run())

	// Strip the existing ad-hoc signature so we can verify codesignBinary adds one.
	strip := exec.Command("codesign", "--remove-signature", bin)
	require.NoError(t, strip.Run())

	err := codesignBinary(bin)
	require.NoError(t, err)

	// Verify the binary is now signed.
	verify := exec.Command("codesign", "--verify", bin)
	assert.NoError(t, verify.Run())
}

func TestAtomicReplaceCallsCodesign(t *testing.T) {
	called := false
	old := codesignFunc
	codesignFunc = func(path string) error {
		called = true
		return nil
	}
	defer func() { codesignFunc = old }()

	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "new")
	require.NoError(t, os.WriteFile(src, []byte("binary"), 0o644))

	dest := filepath.Join(tmpDir, "dottie")
	require.NoError(t, os.WriteFile(dest, []byte("old"), 0o755))

	err := atomicReplace(src, dest)
	require.NoError(t, err)
	assert.True(t, called, "codesignFunc should be called during atomicReplace")
}
