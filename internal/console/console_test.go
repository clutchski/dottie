package console

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestPrinter(verbose bool) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	p := NewWithWriters(&out, &errBuf, verbose)
	return p, &out, &errBuf
}

func TestHeader_VerbosePrintsImmediately(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.Header("dotfiles")
	assert.Equal(t, "dotfiles:\n", out.String())
}

func TestHeader_QuietStoresPending(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("dotfiles")
	assert.Equal(t, "", out.String())
}

func TestHookOk_VerbosePrints(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.HookOk("homebrew", 100*time.Millisecond)
	assert.Equal(t, "  ok homebrew (0.1s)\n", out.String())
}

func TestHookOk_QuietSuppressed(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.HookOk("homebrew", 100*time.Millisecond)
	assert.Equal(t, "", out.String())
}

func TestHookFail_AlwaysPrints(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("hooks pre-link")
	p.HookFail("homebrew", "pre-link", 200*time.Millisecond, "Error: brew not found")
	output := out.String()
	assert.Contains(t, output, "hooks pre-link:\n")
	assert.Contains(t, output, "  FAIL homebrew pre-link hook failed (0.2s)\n")
	assert.Contains(t, output, "    Error: brew not found\n")
}

func TestHookFail_VerbosePrints(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.Header("hooks pre-link")
	p.HookFail("homebrew", "pre-link", 150*time.Millisecond, "brew error")
	output := out.String()
	assert.Contains(t, output, "hooks pre-link:\n")
	assert.Contains(t, output, "  FAIL homebrew pre-link hook failed (0.1s)\n")
	assert.Contains(t, output, "    brew error\n")
}

func TestHookFail_FlushesHeader(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("hooks post-link")
	assert.Equal(t, "", out.String())
	p.HookFail("setup", "post-link", 50*time.Millisecond, "failed")
	assert.Contains(t, out.String(), "hooks post-link:\n")
}

func TestHookFail_MultilineOutput(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.HookFail("test", "status", 100*time.Millisecond, "line1\nline2")
	output := out.String()
	assert.Contains(t, output, "    line1\n")
	assert.Contains(t, output, "    line2\n")
}

func TestDotfileOk_VerbosePrints(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.DotfileOk("vimrc", "~/.vimrc")
	assert.Equal(t, "  ok vimrc -> ~/.vimrc\n", out.String())
}

func TestDotfileOk_QuietSuppressed(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.DotfileOk("vimrc", "~/.vimrc")
	assert.Equal(t, "", out.String())
}

func TestDotfileFail_AlwaysPrints(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("dotfiles")
	p.DotfileFail("vimrc", "not linked")
	output := out.String()
	assert.Contains(t, output, "dotfiles:\n")
	assert.Contains(t, output, "  FAIL vimrc (not linked)\n")
}

func TestDotfileFail_VerbosePrints(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.Header("dotfiles")
	p.DotfileFail("vimrc", "not linked")
	output := out.String()
	assert.Contains(t, output, "dotfiles:\n")
	assert.Contains(t, output, "  FAIL vimrc (not linked)\n")
}

func TestDotfileFail_FlushesHeader(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("dotfiles")
	assert.Equal(t, "", out.String())
	p.DotfileFail("bashrc", "missing")
	assert.Contains(t, out.String(), "dotfiles:\n")
}

func TestErrorf_AlwaysPrintsToStderr(t *testing.T) {
	p, _, errBuf := newTestPrinter(false)
	p.Errorf("something %s", "bad")
	assert.Equal(t, "something bad\n", errBuf.String())
}

func TestErrorf_VerbosePrintsToStderr(t *testing.T) {
	p, _, errBuf := newTestPrinter(true)
	p.Errorf("error: %d", 42)
	assert.Equal(t, "error: 42\n", errBuf.String())
}

func TestHeader_ReplacedByNewHeader(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("first")
	p.Header("second")
	p.DotfileFail("x", "bad")
	output := out.String()
	assert.NotContains(t, output, "first:\n")
	assert.Contains(t, output, "second:\n")
}

func TestNoHeaderFlush_WhenNotSet(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.DotfileFail("x", "bad")
	// Should print the fail line without any header
	assert.Equal(t, "  FAIL x (bad)\n", out.String())
}
