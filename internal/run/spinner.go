package run

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// spinnerFrames goes clockwise then reverses back, creating a bounce effect.
var spinnerFrames = []string{
	"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷",
	"⣯", "⣟", "⡿", "⢿", "⣻",
}

// shimmerGradients are color palettes for hook names. Each hook gets its own
// gradient, cycling through the list. All gradients bounce dark-to-bright-to-dark.
var shimmerGradients = [][]int{
	{130, 166, 172, 208, 214, 220, 214, 208, 172, 166}, // amber/gold
	{37, 38, 44, 45, 51, 87, 51, 45, 44, 38},           // cyan
	{133, 134, 170, 171, 207, 213, 207, 171, 170, 134}, // magenta
	{34, 35, 41, 42, 48, 84, 48, 42, 41, 35},           // green
}

// pulseGradient is the spinner character's color cycle (amber).
var pulseGradient = shimmerGradients[0]

// Spinner renders an animated progress line on a TTY. No-ops when not a TTY.
type Spinner struct {
	out          io.Writer
	isTTY        bool
	startTime    time.Time
	pulseEnabled bool
}

// NewSpinner creates a Spinner. When isTTY is false, all methods are no-ops.
func NewSpinner(out io.Writer, isTTY bool) *Spinner {
	return &Spinner{
		out:          out,
		isTTY:        isTTY,
		startTime:    time.Now(),
		pulseEnabled: true,
	}
}

// Render draws the spinner line for the given phase and active hooks.
// The entire line is buffered and written in a single call to avoid flicker.
func (s *Spinner) Render(phase Phase, active []string) {
	if !s.isTTY {
		return
	}

	label, names := buildActivity(phase, active)
	if label == "" {
		return
	}

	frame := int(time.Since(s.startTime).Milliseconds() / 120)

	var b strings.Builder
	b.WriteString("\r")
	b.WriteString(s.pulseColor(frame, spinnerFrames[frame%len(spinnerFrames)]))
	b.WriteString(" ")
	fmt.Fprintf(&b, "\033[90mdottie %s\033[0m", label)
	if len(names) > 0 {
		b.WriteString(" ")
		s.writeShimmerNames(&b, frame, names)
	}
	b.WriteString("\033[K")

	fmt.Fprint(s.out, b.String())
}

// Clear erases the spinner line.
func (s *Spinner) Clear() {
	if !s.isTTY {
		return
	}
	fmt.Fprint(s.out, "\r\033[K")
}

func buildActivity(phase Phase, active []string) (label string, names []string) {
	switch phase {
	case PhasePreHooks, PhasePostHooks:
		return "is hooking up", active
	case PhaseLinking:
		return "is linking", nil
	}
	return "", nil
}

func (s *Spinner) pulseColor(frame int, text string) string {
	if !s.pulseEnabled {
		return text
	}
	code := pulseGradient[(frame/2)%len(pulseGradient)]
	return fmt.Sprintf("\033[38;5;%dm%s\033[0m", code, text)
}

// writeShimmerNames writes each hook name with its own color gradient wave,
// separated by grey commas.
func (s *Spinner) writeShimmerNames(b *strings.Builder, frame int, names []string) {
	if !s.pulseEnabled {
		b.WriteString(strings.Join(names, ", "))
		return
	}
	for i, name := range names {
		if i > 0 {
			b.WriteString("\033[90m, \033[0m")
		}
		grad := shimmerGradients[i%len(shimmerGradients)]
		n := len(grad)
		for j, r := range name {
			code := grad[((frame/2)+j)%n]
			fmt.Fprintf(b, "\033[38;5;%dm%c", code, r)
		}
		b.WriteString("\033[0m")
	}
}
