package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigLocation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		local    bool
		override string
		want     string
		fail     bool
	}{
		{name: "default", want: ".dotfiles"},
		{name: "current directory", local: true, want: "cwd"},
		{name: "environment overrides current", local: true, override: "custom", want: "custom"},
		{name: "relative environment", override: "../custom", want: "custom"},
		{name: "invalid override does not fall back", override: "missing", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			t.Setenv("HOME", base)
			t.Setenv("DOTFILES_DIR", "")
			cwd, err := os.Getwd()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })
			for _, dir := range []string{"cwd", ".dotfiles", "custom"} {
				require.NoError(t, os.MkdirAll(filepath.Join(base, dir), 0o755))
				if dir != "cwd" || tc.local {
					require.NoError(t, os.Mkdir(filepath.Join(base, dir, "home"), 0o755))
				}
			}
			require.NoError(t, os.Chdir(filepath.Join(base, "cwd")))
			if tc.override != "" {
				root := tc.override
				if root != "../custom" {
					root = filepath.Join(base, root)
				}
				t.Setenv("DOTFILES_DIR", root)
			}
			cfg, err := loadConfig()
			if tc.fail {
				require.ErrorContains(t, err, "DOTFILES_DIR")
				return
			}
			require.NoError(t, err)
			require.Equal(t, filepath.Join(base, tc.want), cfg.RepoRoot())
		})
	}
}

func TestRunFromOutsideRepository(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	root := filepath.Join(base, ".dotfiles")
	target := filepath.Join(base, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "home"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "hooks"), 0o755))
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "home", "testrc"), []byte("config"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dottie.yaml"), []byte("target_dir: "+target+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "hooks", "check.sh"), []byte("#!/bin/sh\n[ -f \"$DOTTIE_ROOT/home/testrc\" ]\n"), 0o755))
	t.Setenv("HOME", base)
	t.Setenv("DOTFILES_DIR", "")
	require.Equal(t, 0, Run([]string{"run", "--no-progress"}))
	dest, err := os.Readlink(filepath.Join(target, ".testrc"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "home", "testrc"), dest)
}
