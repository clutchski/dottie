package console

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

var (
	progressGrey = color.New(color.FgHiBlack)

	// pulseGradient cycles ANSI-256 colors from dark amber to bright gold and back.
	pulseGradient = []int{130, 166, 172, 208, 214, 220, 214, 208, 172, 166}
	pulseEnabled  = true

	// taskPalette is a set of bright colors assigned to hook names.
	taskPalette = []*color.Color{
		color.New(color.FgHiCyan),
		color.New(color.FgHiMagenta),
		color.New(color.FgHiYellow),
		color.New(color.FgHiGreen),
		color.New(color.FgHiBlue),
		color.New(color.FgHiRed),
	}
)

// disableProgressColor turns off color for tests.
func disableProgressColor() {
	pulseEnabled = false
	progressGrey.DisableColor()
	for _, c := range taskPalette {
		c.DisableColor()
	}
}

// pulseColor wraps text in an ANSI-256 color from the pulse gradient.
func pulseColor(frame int, text string) string {
	if !pulseEnabled {
		return text
	}
	code := pulseGradient[frame%len(pulseGradient)]
	return fmt.Sprintf("\033[38;5;%dm%s\033[0m", code, text)
}

// Progress manages an in-place status line on a TTY.
// When isTTY is false, all methods are no-ops.
type Progress struct {
	out        io.Writer
	mu         sync.Mutex
	active     []string
	taskLabel  string
	taskColors map[string]*color.Color
	message    string
	frame      int
	ticker     *time.Ticker
	done       chan struct{}
	isTTY      bool
	stopped    bool
}

// NewProgress creates a new Progress. When isTTY is false, all methods are
// no-ops and nothing is written.
func NewProgress(out io.Writer, isTTY bool) *Progress {
	p := &Progress{
		out:   out,
		isTTY: isTTY,
		done:  make(chan struct{}),
	}
	if isTTY {
		p.ticker = time.NewTicker(100 * time.Millisecond)
		go p.loop()
	}
	return p
}

// SetTasks sets the list of active task names with a label (e.g. "hooks:pre").
// Each task is assigned a color from the palette, cycling through available colors.
// Resets the ticker so the new phase renders immediately.
func (p *Progress) SetTasks(label string, names []string) {
	if !p.isTTY {
		return
	}
	p.mu.Lock()
	p.taskLabel = label
	p.active = make([]string, len(names))
	copy(p.active, names)
	p.taskColors = make(map[string]*color.Color, len(names))
	for i, name := range names {
		p.taskColors[name] = taskPalette[i%len(taskPalette)]
	}
	p.message = ""
	p.mu.Unlock()
	p.ticker.Reset(100 * time.Millisecond)
}

// FinishTask removes a task from the active list.
func (p *Progress) FinishTask(name string) {
	if !p.isTTY {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, n := range p.active {
		if n == name {
			p.active = append(p.active[:i], p.active[i+1:]...)
			return
		}
	}
}

// SetMessage sets a static message, replacing any active task list.
// Resets the ticker so the new message renders immediately.
func (p *Progress) SetMessage(msg string) {
	if !p.isTTY {
		return
	}
	p.mu.Lock()
	p.message = msg
	p.active = nil
	p.mu.Unlock()
	p.ticker.Reset(100 * time.Millisecond)
}

// Clear clears the progress line so other output can print cleanly.
// The spinner resumes on the next tick.
func (p *Progress) Clear() {
	if !p.isTTY {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLine()
}

// Stop stops the ticker, clears the line, and prepares for final output.
func (p *Progress) Stop() {
	if !p.isTTY {
		return
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	p.mu.Unlock()

	p.ticker.Stop()
	close(p.done)
	p.clearLine()
}

func (p *Progress) loop() {
	for {
		select {
		case <-p.done:
			return
		case <-p.ticker.C:
			p.render()
		}
	}
}

func (p *Progress) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	activity := p.buildActivity()
	if activity == "" {
		return
	}

	spinner := pulseColor(p.frame/2, spinnerFrames[p.frame%len(spinnerFrames)])
	sep := progressGrey.Sprint("·")
	p.frame++

	fmt.Fprintf(p.out, "\r%s dottie %s %s\033[K", spinner, sep, activity)
}

func (p *Progress) buildActivity() string {
	if p.message != "" {
		return p.message
	}
	if len(p.active) > 0 {
		colored := make([]string, len(p.active))
		for i, name := range p.active {
			if c, ok := p.taskColors[name]; ok {
				colored[i] = c.Sprint(name)
			} else {
				colored[i] = name
			}
		}
		return "running " + p.taskLabel + " " + strings.Join(colored, progressGrey.Sprint(", "))
	}
	return ""
}

func (p *Progress) clearLine() {
	fmt.Fprint(p.out, "\r\033[K")
}
