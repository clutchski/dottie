package run

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpinner_RenderShowsPhaseAndActive(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.pulseEnabled = false

	s.Render(PhasePreHooks, []string{"homebrew", "apt"})

	out := buf.String()
	assert.Contains(t, out, "dottie is hooking up")
	assert.Contains(t, out, "homebrew, apt")
}

func TestSpinner_RenderShowsLinking(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.pulseEnabled = false

	s.Render(PhaseLinking, nil)

	out := buf.String()
	assert.Contains(t, out, "dottie is linking")
}

func TestSpinner_RenderShowsPostHooks(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.pulseEnabled = false

	s.Render(PhasePostHooks, []string{"cleanup"})

	out := buf.String()
	assert.Contains(t, out, "dottie is hooking up")
	assert.Contains(t, out, "cleanup")
}

func TestSpinner_ClearWritesEscapeSequence(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)

	s.Clear()

	assert.Equal(t, "\r\033[K", buf.String())
}

func TestSpinner_NoOpWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, false)

	s.Render(PhasePreHooks, []string{"test"})
	s.Clear()

	assert.Empty(t, buf.String())
}

func TestSpinner_FrameAdvancesOverTime(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.pulseEnabled = false

	s.Render(PhasePreHooks, []string{"test"})
	first := buf.String()
	buf.Reset()

	time.Sleep(150 * time.Millisecond)
	s.Render(PhasePreHooks, []string{"test"})
	second := buf.String()

	assert.NotEqual(t, first, second, "spinner frame should advance over time")
}

func TestSpinner_HookPhaseNoActiveStillRenders(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.pulseEnabled = false

	s.Render(PhasePreHooks, nil)

	out := buf.String()
	assert.Contains(t, out, "dottie is hooking up")
}

func TestSpinner_NoOutputForIdlePhase(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)

	s.Render(PhaseIdle, nil)

	assert.Empty(t, buf.String())
}

func TestSpinner_NoOutputForDonePhase(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)

	s.Render(PhaseDone, nil)

	assert.Empty(t, buf.String())
}

func TestSpinner_ClearBeforeRender(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.pulseEnabled = false

	s.Render(PhasePreHooks, []string{"test"})

	out := buf.String()
	assert.True(t, strings.HasPrefix(out, "\r"), "should start with carriage return")
}
