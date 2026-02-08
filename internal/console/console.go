package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
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

// PrintHook prints a hook result.
func (p *Printer) PrintHook(r hooks.HookResult, phase string) {
	if r.Ok() {
		p.hookOk(r.Name, r.Elapsed)
	} else {
		p.hookFail(r.Name, phase, r.Elapsed, r.Output)
	}
}

// PrintLink prints a link result.
func (p *Printer) PrintLink(r link.Result) {
	switch r.Status {
	case link.StatusLinked, link.StatusAlreadyLinked, link.StatusWouldLink:
		p.dotfileOk(r.Name, r.Target)
	case link.StatusError:
		msg := "error"
		if r.Error != nil {
			msg = r.Error.Error()
		}
		p.dotfileFail(r.Name, msg)
	case link.StatusMissing:
		p.dotfileFail(r.Name, r.Message)
	case link.StatusDiff:
		p.dotfileFail(r.Name, r.Message)
	}
}

// PrintHookStatus prints a hook status check result.
func (p *Printer) PrintHookStatus(s hooks.HookStatus) {
	name := hooks.DisplayName(s.Name)
	if s.Ok() {
		p.hookOk(name, 0)
	} else {
		p.dotfileFail(name, "hook failed")
	}
}

// Errorf prints an error message to stderr. Always prints.
func (p *Printer) Errorf(format string, args ...any) {
	fmt.Fprintf(p.err, format+"\n", args...)
}

func (p *Printer) hookOk(name string, d time.Duration) {
	if !p.verbose {
		return
	}
	fmt.Fprintf(p.out, "  ok %s (%.1fs)\n", name, d.Seconds())
}

func (p *Printer) hookFail(name, phase string, d time.Duration, output string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  FAIL %s %s hook failed (%.1fs)\n", name, phase, d.Seconds())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			fmt.Fprintf(p.out, "    %s\n", line)
		}
	}
}

func (p *Printer) dotfileOk(name, target string) {
	if !p.verbose {
		return
	}
	fmt.Fprintf(p.out, "  ok %s -> %s\n", name, target)
}

func (p *Printer) dotfileFail(name, msg string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  FAIL %s (%s)\n", name, msg)
}
