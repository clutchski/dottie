package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Printer handles all CLI output formatting.
// In verbose mode, headers print immediately and ok results are shown.
// In quiet mode, headers are lazy (only printed when a failure follows)
// and ok results are suppressed.
type Printer struct {
	out           io.Writer
	err           io.Writer
	verbose       bool
	pendingHeader string
}

// New creates a Printer that writes to stdout/stderr.
func New(verbose bool) *Printer {
	return &Printer{
		out:     os.Stdout,
		err:     os.Stderr,
		verbose: verbose,
	}
}

// NewWithWriters creates a Printer with custom writers (for testing).
func NewWithWriters(out, err io.Writer, verbose bool) *Printer {
	return &Printer{
		out:     out,
		err:     err,
		verbose: verbose,
	}
}

// Header prints a section header. In verbose mode it prints immediately.
// In quiet mode it stores the header and only prints it if a failure follows.
func (p *Printer) Header(name string) {
	if p.verbose {
		fmt.Fprintf(p.out, "%s:\n", name)
		return
	}
	p.pendingHeader = name
}

// flushHeader prints and clears any pending header.
func (p *Printer) flushHeader() {
	if p.pendingHeader != "" {
		fmt.Fprintf(p.out, "%s:\n", p.pendingHeader)
		p.pendingHeader = ""
	}
}

// HookOk prints a successful hook result. Verbose only.
func (p *Printer) HookOk(name string, d time.Duration) {
	if !p.verbose {
		return
	}
	fmt.Fprintf(p.out, "  ok %s (%.1fs)\n", name, d.Seconds())
}

// HookFail prints a failed hook result. Always prints, flushes pending header.
func (p *Printer) HookFail(name, phase string, d time.Duration, output string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  FAIL %s %s hook failed (%.1fs)\n", name, phase, d.Seconds())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			fmt.Fprintf(p.out, "    %s\n", line)
		}
	}
}

// DotfileOk prints a successfully linked dotfile. Verbose only.
func (p *Printer) DotfileOk(name, target string) {
	if !p.verbose {
		return
	}
	fmt.Fprintf(p.out, "  ok %s -> %s\n", name, target)
}

// DotfileFail prints a dotfile problem. Always prints, flushes pending header.
func (p *Printer) DotfileFail(name, msg string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  FAIL %s (%s)\n", name, msg)
}

// Errorf prints an error message to stderr. Always prints.
func (p *Printer) Errorf(format string, args ...any) {
	fmt.Fprintf(p.err, format+"\n", args...)
}
