package console

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
	"github.com/stretchr/testify/assert"
)

func newTestPrinter(verbose bool) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	p := NewWithWriters(&out, &errBuf, verbose)
	return p, &out, &errBuf
}

// --- PrintHook ---

func TestPrintHook_OkVerbose(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 0, Elapsed: 100 * time.Millisecond}, "pre-link")
	assert.Equal(t, "  ok homebrew (0.1s)\n", out.String())
}

func TestPrintHook_OkQuiet(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 0, Elapsed: 100 * time.Millisecond}, "pre-link")
	assert.Equal(t, "", out.String())
}

func TestPrintHook_FailVerbose(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.Header("hooks pre-link")
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 1, Elapsed: 200 * time.Millisecond, Output: "brew not found"}, "pre-link")
	output := out.String()
	assert.Contains(t, output, "  FAIL homebrew pre-link hook failed (0.2s)")
	assert.Contains(t, output, "    brew not found")
}

func TestPrintHook_FailQuietFlushesHeader(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("hooks pre-link")
	p.PrintHook(hooks.HookResult{Name: "setup", ExitCode: 42, Elapsed: 50 * time.Millisecond, Output: "error"}, "pre-link")
	output := out.String()
	assert.Contains(t, output, "hooks pre-link:\n")
	assert.Contains(t, output, "  FAIL setup pre-link hook failed")
}

// --- PrintLink ---

func TestPrintLink_Linked(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusLinked})
	assert.Equal(t, "  ok vimrc -> ~/.vimrc\n", out.String())
}

func TestPrintLink_AlreadyLinked(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusAlreadyLinked})
	assert.Equal(t, "  ok vimrc -> ~/.vimrc\n", out.String())
}

func TestPrintLink_WouldLink(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusWouldLink})
	assert.Equal(t, "  ok vimrc -> ~/.vimrc\n", out.String())
}

func TestPrintLink_OkQuietSuppressed(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusLinked})
	assert.Equal(t, "", out.String())
}

func TestPrintLink_Error(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("dotfiles")
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusError, Error: fmt.Errorf("permission denied")})
	output := out.String()
	assert.Contains(t, output, "dotfiles:\n")
	assert.Contains(t, output, "  FAIL vimrc (permission denied)")
}

func TestPrintLink_ErrorNilError(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusError})
	assert.Contains(t, out.String(), "  FAIL vimrc (error)")
}

func TestPrintLink_Missing(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusMissing, Message: "not linked"})
	assert.Contains(t, out.String(), "  FAIL vimrc (not linked)")
}

func TestPrintLink_Diff(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusDiff, Message: "symlink points to /other"})
	assert.Contains(t, out.String(), "  FAIL vimrc (symlink points to /other)")
}

// --- PrintHookStatus ---

func TestPrintHookStatus_Ok(t *testing.T) {
	p, out, _ := newTestPrinter(true)
	p.PrintHookStatus(hooks.HookStatus{Name: "homebrew.sh", ExitCode: 0})
	assert.Equal(t, "  ok homebrew (0.0s)\n", out.String())
}

func TestPrintHookStatus_Fail(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("hooks")
	p.PrintHookStatus(hooks.HookStatus{Name: "homebrew.sh", ExitCode: 1})
	output := out.String()
	assert.Contains(t, output, "hooks:\n")
	assert.Contains(t, output, "  FAIL homebrew (hook failed)")
}

// --- Lazy header ---

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

func TestHeader_ReplacedByNewHeader(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Header("first")
	p.Header("second")
	p.PrintLink(link.Result{Name: "x", Status: link.StatusMissing, Message: "bad"})
	output := out.String()
	assert.NotContains(t, output, "first:\n")
	assert.Contains(t, output, "second:\n")
}

func TestNoHeaderFlush_WhenNotSet(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.PrintLink(link.Result{Name: "x", Status: link.StatusMissing, Message: "bad"})
	assert.Equal(t, "  FAIL x (bad)\n", out.String())
}

// --- Summary ---

func TestSummary_AllOk(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Summary(3, 3, 30, 30, 2, 2)
	assert.Equal(t, "✓ dottie   hooks:pre 3/3   links 30/30   hooks:post 2/2\n", out.String())
}

func TestSummary_Failures(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Summary(2, 3, 28, 30, 1, 2)
	assert.Equal(t, "✗ dottie   hooks:pre 2/3   links 28/30   hooks:post 1/2\n", out.String())
}

func TestSummary_NoHooks(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Summary(0, 0, 12, 12, 0, 0)
	assert.Equal(t, "✓ dottie   links 12/12\n", out.String())
}

func TestSummary_PreOnly(t *testing.T) {
	p, out, _ := newTestPrinter(false)
	p.Summary(2, 2, 10, 10, 0, 0)
	assert.Equal(t, "✓ dottie   hooks:pre 2/2   links 10/10\n", out.String())
}

// --- Errorf ---

func TestErrorf_AlwaysPrintsToStderr(t *testing.T) {
	p, _, errBuf := newTestPrinter(false)
	p.Errorf("something %s", "bad")
	assert.Equal(t, "something bad\n", errBuf.String())
}
