package console

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func newTestPrinter(v Verbosity) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	color.NoColor = true
	var out, errBuf bytes.Buffer
	p := NewWithWriters(&out, &errBuf, v)
	return p, &out, &errBuf
}

// --- PrintHook ---

func TestPrintHook_OkVerbose(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 0, Elapsed: 100 * time.Millisecond}, "pre-link")
	assert.Equal(t, "  ✓ homebrew (0.1s)\n", out.String())
}

func TestPrintHook_OkQuiet(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 0, Elapsed: 100 * time.Millisecond}, "pre-link")
	assert.Empty(t, out.String())
}

func TestPrintHook_FailVerbose(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.Header("hooks pre-link")
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 1, Elapsed: 200 * time.Millisecond, Output: "brew not found"}, "pre-link")
	output := out.String()
	assert.Contains(t, output, "  ✗ homebrew pre-link hook failed (0.2s)")
	assert.Contains(t, output, "    | brew not found")
}

func TestPrintHook_FailQuietFlushesHeader(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Header("hooks pre-link")
	p.PrintHook(hooks.HookResult{Name: "setup", ExitCode: 42, Elapsed: 50 * time.Millisecond, Output: "error"}, "pre-link")
	output := out.String()
	assert.Contains(t, output, "hooks pre-link:\n")
	assert.Contains(t, output, "  ✗ setup pre-link hook failed")
}

// --- PrintLink ---

func TestPrintLink_Linked(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusLinked})
	assert.Equal(t, "  ✓ vimrc -> ~/.vimrc\n", out.String())
}

func TestPrintLink_AlreadyLinked(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusAlreadyLinked})
	assert.Equal(t, "  ✓ vimrc -> ~/.vimrc\n", out.String())
}

func TestPrintLink_WouldLink(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusWouldLink})
	assert.Equal(t, "  ✓ vimrc -> ~/.vimrc\n", out.String())
}

func TestPrintLink_OkQuietSuppressed(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintLink(link.Result{Name: "vimrc", Target: "~/.vimrc", Status: link.StatusLinked})
	assert.Empty(t, out.String())
}

func TestPrintLink_Error(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Header("links")
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusError, Error: fmt.Errorf("permission denied")})
	output := out.String()
	assert.Contains(t, output, "links:\n")
	assert.Contains(t, output, "  ✗ vimrc (permission denied)")
}

func TestPrintLink_ErrorNilError(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusError})
	assert.Contains(t, out.String(), "  ✗ vimrc (error)")
}

func TestPrintLink_Missing(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusMissing, Message: "not linked"})
	assert.Contains(t, out.String(), "  ✗ vimrc (not linked)")
}

func TestPrintLink_Diff(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintLink(link.Result{Name: "vimrc", Status: link.StatusDiff, Message: "symlink points to /other"})
	assert.Contains(t, out.String(), "  ✗ vimrc (symlink points to /other)")
}

func TestPrintLink_DanglingVerbose(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.Header("links")
	p.PrintLink(link.Result{Name: ".vimrc", Status: link.StatusDangling})
	assert.Contains(t, out.String(), "  ✗ .vimrc (orphan)")
}

func TestPrintLink_DanglingQuietSuppressed(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintLink(link.Result{Name: ".vimrc", Status: link.StatusDangling})
	assert.Empty(t, out.String())
}

// --- PrintHookStatus ---

func TestPrintHookStatus_Ok(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintHookStatus(hooks.HookStatus{Name: "homebrew.sh", ExitCode: 0})
	assert.Equal(t, "  ✓ homebrew (0.0s)\n", out.String())
}

func TestPrintHookStatus_Fail(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Header("hooks")
	p.PrintHookStatus(hooks.HookStatus{Name: "homebrew.sh", ExitCode: 2})
	output := out.String()
	assert.Contains(t, output, "hooks:\n")
	assert.Contains(t, output, "  ✗ homebrew (hook failed)")
}

func TestPrintHookStatus_NeedsUpdate(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintHookStatus(hooks.HookStatus{Name: "homebrew.sh", ExitCode: 1, Output: "outdated"})
	assert.Contains(t, out.String(), "~ homebrew (needs update)")
}

func TestPrintHook_StatusPhaseNeedsUpdate(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 1, Elapsed: 100 * time.Millisecond, Output: "outdated"}, "status")
	assert.Contains(t, out.String(), "~ homebrew (needs update)")
}

func TestPrintHook_Everything_ShowsOutput(t *testing.T) {
	p, out, _ := newTestPrinter(Everything)
	p.PrintHook(hooks.HookResult{Name: "homebrew", ExitCode: 0, Elapsed: 100 * time.Millisecond, Output: "all good\n"}, "pre-link")
	output := out.String()
	assert.Contains(t, output, "homebrew (0.1s)")
	assert.Contains(t, output, "    | all good")
}

func TestPrintHook_Everything_ShowsMultilineOutput(t *testing.T) {
	p, out, _ := newTestPrinter(Everything)
	p.PrintHook(hooks.HookResult{
		Name:     "brew",
		ExitCode: 0,
		Elapsed:  100 * time.Millisecond,
		Output:   "line one\nline two\n",
	}, "pre-link")
	output := out.String()
	assert.Contains(t, output, "    | line one\n")
	assert.Contains(t, output, "    | line two\n")
}

func TestPrintHook_Verbose_DoesNotShowOutput(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintHook(hooks.HookResult{
		Name:     "brew",
		ExitCode: 0,
		Elapsed:  100 * time.Millisecond,
		Output:   "should not appear\n",
	}, "pre-link")
	output := out.String()
	assert.NotContains(t, output, "should not appear")
}

func TestPrintHookStatus_Everything_ShowsOutput(t *testing.T) {
	p, out, _ := newTestPrinter(Everything)
	p.PrintHookStatus(hooks.HookStatus{Name: "homebrew.sh", ExitCode: 0, Output: "all good\n"})
	output := out.String()
	assert.Contains(t, output, "homebrew (0.0s)")
	assert.Contains(t, output, "    | all good")
}

// --- Lazy header ---

func TestHeader_VerbosePrintsImmediately(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.Header("links")
	assert.Equal(t, "links:\n", out.String())
}

func TestHeader_QuietStoresPending(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Header("links")
	assert.Empty(t, out.String())
}

func TestHeader_ReplacedByNewHeader(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Header("first")
	p.Header("second")
	p.PrintLink(link.Result{Name: "x", Status: link.StatusMissing, Message: "bad"})
	output := out.String()
	assert.NotContains(t, output, "first:\n")
	assert.Contains(t, output, "second:\n")
}

func TestNoHeaderFlush_WhenNotSet(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.PrintLink(link.Result{Name: "x", Status: link.StatusMissing, Message: "bad"})
	assert.Equal(t, "  ✗ x (bad)\n", out.String())
}

// --- Summary ---

func TestSummary_AllUnchanged(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(3, 3, LinkSummary{Existing: 30}, 2, 2)
	assert.Equal(t, "✓ dottie · hooks:pre 3 · links 30 · hooks:post 2\n", out.String())
}

func TestSummary_WithAdded(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(3, 3, LinkSummary{Existing: 28, Added: 2}, 2, 2)
	assert.Equal(t, "✓ dottie · hooks:pre 3 · links 28 +2 · hooks:post 2\n", out.String())
}

func TestSummary_WithPruned(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(3, 3, LinkSummary{Existing: 30, Pruned: 3}, 2, 2)
	assert.Equal(t, "✓ dottie · hooks:pre 3 · links 30 -3 · hooks:post 2\n", out.String())
}

func TestSummary_WithAddedAndPruned(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(3, 3, LinkSummary{Existing: 35, Added: 3, Pruned: 2}, 2, 2)
	assert.Equal(t, "✓ dottie · hooks:pre 3 · links 35 +3 -2 · hooks:post 2\n", out.String())
}

func TestSummary_WithErrors(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(2, 3, LinkSummary{Existing: 28, Errors: 2}, 1, 2)
	assert.Equal(t, "✗ dottie · hooks:pre 2/3 · links 28 !2 · hooks:post 1/2\n", out.String())
}

func TestSummary_NoHooks(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(0, 0, LinkSummary{Existing: 12}, 0, 0)
	assert.Equal(t, "✓ dottie · links 12\n", out.String())
}

func TestSummary_PreOnly(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.Summary(2, 2, LinkSummary{Existing: 10}, 0, 0)
	assert.Equal(t, "✓ dottie · hooks:pre 2 · links 10\n", out.String())
}

// --- PrintDottieStatus ---

func TestPrintDottieStatus_UpToDate(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintDottieStatus("/usr/local/bin/dottie", "/home/matt/dotfiles/dottie.yaml", "v1.2.3", "", true)
	output := out.String()
	assert.Contains(t, output, "dottie:\n")
	assert.Contains(t, output, "  binary: /usr/local/bin/dottie\n")
	assert.Contains(t, output, "  config: /home/matt/dotfiles/dottie.yaml\n")
	assert.Contains(t, output, "  version: v1.2.3\n")
	assert.NotContains(t, output, "update")
}

func TestPrintDottieStatus_UpdateAvailable(t *testing.T) {
	p, out, _ := newTestPrinter(Verbose)
	p.PrintDottieStatus("/usr/local/bin/dottie", "/home/matt/dotfiles/dottie.yaml", "v1.2.3", "v1.3.0", false)
	output := out.String()
	assert.Contains(t, output, "  version: v1.2.3 (update available: v1.3.0)\n")
}

// --- StatusSummary ---

func TestStatusSummary_AllOk(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.StatusSummary(
		LinkCounts{Ok: 34},
		HookCounts{Ok: 5},
		"v1.2.3", "", true,
	)
	output := out.String()
	assert.Equal(t, "links: ok:34\nhooks: ok:5\ndottie v1.2.3\n", output)
}

func TestStatusSummary_WithIssues(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.StatusSummary(
		LinkCounts{Ok: 32, Missing: 2, Diff: 1},
		HookCounts{Ok: 2, Update: 3, Err: 1},
		"v1.2.3", "v1.3.0", false,
	)
	output := out.String()
	assert.Equal(t, "links: ok:32 · unlinked:2 · diff:1\nhooks: ok:2 · update:3 · err:1\ndottie v1.2.3 (update available: v1.3.0)\n", output)
}

func TestStatusSummary_NoHooks(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.StatusSummary(
		LinkCounts{Ok: 34},
		HookCounts{},
		"v1.2.3", "", true,
	)
	output := out.String()
	assert.Equal(t, "links: ok:34\ndottie v1.2.3\n", output)
}

func TestStatusSummary_NoHooksUpdateAvailable(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.StatusSummary(
		LinkCounts{Ok: 10, Missing: 1},
		HookCounts{},
		"v1.2.3", "v1.3.0", false,
	)
	output := out.String()
	assert.Equal(t, "links: ok:10 · unlinked:1\ndottie v1.2.3 (update available: v1.3.0)\n", output)
}

func TestStatusSummary_WithDangling(t *testing.T) {
	p, out, _ := newTestPrinter(Quiet)
	p.StatusSummary(
		LinkCounts{Ok: 34, Dangling: 2},
		HookCounts{Ok: 5},
		"v1.2.3", "", true,
	)
	output := out.String()
	assert.Equal(t, "links: ok:34 · orphan:2\nhooks: ok:5\ndottie v1.2.3\n", output)
}

// --- Errorf ---

func TestErrorf_AlwaysPrintsToStderr(t *testing.T) {
	p, _, errBuf := newTestPrinter(Quiet)
	p.Errorf("something %s", "bad")
	assert.Equal(t, "something bad\n", errBuf.String())
}
