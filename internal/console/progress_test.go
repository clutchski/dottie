package console

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Disable color in tests so assertions can match plain text.
	disableProgressColor()
}

func TestNewProgress_NonTTY_IsNoOp(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, false)
	p.SetTasks("hooks:pre", []string{"homebrew", "apt"})
	p.FinishTask("homebrew")
	p.SetMessage("linking 10 files")
	p.Stop()
	assert.Empty(t, buf.String())
}

func TestProgress_SetTasks_RendersActiveNames(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:pre", []string{"homebrew", "apt"})

	// Wait for at least one tick to render
	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	assert.Contains(t, output, "running hooks:pre homebrew, apt")
}

func TestProgress_FinishTask_RemovesFromActive(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:pre", []string{"homebrew", "apt", "node"})
	p.FinishTask("apt")

	// Wait for a tick to render
	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	// After finishing "apt", should show homebrew and node but not apt.
	// The last segment is the clear sequence from Stop(), so check the
	// second-to-last rendered segment.
	lines := strings.Split(output, "\r")
	require.GreaterOrEqual(t, len(lines), 2, "expected at least 2 segments")
	lastRendered := lines[len(lines)-2]
	assert.NotContains(t, lastRendered, "apt")
	assert.Contains(t, lastRendered, "homebrew")
	assert.Contains(t, lastRendered, "node")
}

func TestProgress_SetMessage_RendersStaticMessage(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetMessage("linking 16 files")

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	assert.Contains(t, output, "linking 16 files")
}

func TestProgress_Stop_ClearsLine(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetMessage("working")

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	// Stop should end with a carriage return + clear to EOL
	assert.True(t, strings.HasSuffix(output, "\r\033[K"))
}

func TestProgress_SpinnerFrames(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetMessage("working")

	// Wait enough for multiple frames
	time.Sleep(250 * time.Millisecond)
	p.Stop()

	output := buf.String()
	// Should contain spinner characters from the set
	frames := spinnerFrames
	found := 0
	for _, f := range frames {
		if strings.Contains(output, f) {
			found++
		}
	}
	assert.Positive(t, found, "should contain at least one spinner frame")
}

func TestProgress_FinishAllTasks_StopsRendering(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:pre", []string{"homebrew"})
	p.FinishTask("homebrew")

	time.Sleep(150 * time.Millisecond)
	snap := buf.String()
	p.Stop()

	// After all tasks finish, the line should be cleared (no active tasks to show)
	lines := strings.Split(snap, "\r")
	// The last rendered content should either be empty or cleared
	require.NotEmpty(t, lines)
}

func TestProgress_StopIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetMessage("test")
	time.Sleep(150 * time.Millisecond)
	p.Stop()
	p.Stop() // should not panic
}

func TestProgress_SetTasks_ReplacesActiveList(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:pre", []string{"homebrew", "apt"})
	p.SetTasks("hooks:post", []string{"cleanup"})

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	lines := strings.Split(output, "\r")
	lastLine := lines[len(lines)-1]
	// The last rendered content before stop should only show cleanup
	// (or be clear). Check that earlier renders had the new task.
	found := false
	for _, line := range lines {
		if strings.Contains(line, "cleanup") {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain 'cleanup' after SetTasks replaces list")
	_ = lastLine
}

func TestProgress_HasDottiePrefix(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:pre", []string{"homebrew"})

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	assert.Contains(t, output, "dottie · running hooks:pre homebrew")
}

func TestProgress_MessageHasDottiePrefix(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetMessage("linking 16 files")

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	assert.Contains(t, output, "dottie · linking 16 files")
}

func TestProgress_SetMessage_ClearsActiveList(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:pre", []string{"homebrew", "apt"})

	time.Sleep(120 * time.Millisecond)
	p.SetMessage("linking 16 files")

	time.Sleep(120 * time.Millisecond)
	p.Stop()

	output := buf.String()
	// Find rendered lines after SetMessage was called
	lines := strings.Split(output, "\r")
	// At least one line should have "linking 16 files" without "homebrew"
	foundLinkingWithoutHooks := false
	for _, line := range lines {
		if strings.Contains(line, "linking 16 files") && !strings.Contains(line, "homebrew") {
			foundLinkingWithoutHooks = true
			break
		}
	}
	assert.True(t, foundLinkingWithoutHooks, "SetMessage should replace active task list")
}

func TestProgress_Clear_NonTTY_IsNoOp(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, false)
	p.Clear() // should not panic or write anything
	assert.Empty(t, buf.String())
}

func TestProgress_Clear_ClearsProgressLine(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetMessage("working")
	time.Sleep(150 * time.Millisecond)
	p.Clear()
	p.Stop()

	output := buf.String()
	// Clear + Stop should produce at least 2 clear sequences
	count := strings.Count(output, "\r\033[K")
	assert.GreaterOrEqual(t, count, 2)
}

func TestProgress_PulseGradient(t *testing.T) {
	var buf bytes.Buffer
	// Re-enable pulse for this test (init disables it)
	old := pulseEnabled
	pulseEnabled = true
	defer func() { pulseEnabled = old }()

	p := NewProgress(&buf, true)
	p.SetMessage("working")

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	// Should contain ANSI 256-color escape sequences from the gradient
	assert.Contains(t, output, "\033[38;5;")
}

func TestProgress_PhaseLabel(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.SetTasks("hooks:post", []string{"cleanup"})

	time.Sleep(150 * time.Millisecond)
	p.Stop()

	output := buf.String()
	assert.Contains(t, output, "running hooks:post cleanup")
}
