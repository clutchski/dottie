package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
	"github.com/fatih/color"
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
	green         *color.Color
	red           *color.Color
	bold          *color.Color
	hiGreen       *color.Color
	hiRed         *color.Color
	yellow        *color.Color
	grey          *color.Color
}

// New creates a Printer that writes to stdout/stderr.
func New(verbose bool) *Printer {
	return &Printer{
		out:     os.Stdout,
		err:     os.Stderr,
		verbose: verbose,
		green:   color.New(color.FgGreen),
		red:     color.New(color.FgRed),
		bold:    color.New(color.Bold),
		hiGreen: color.New(color.FgHiGreen),
		hiRed:   color.New(color.FgHiRed),
		yellow:  color.New(color.FgYellow),
		grey:    color.New(color.FgHiBlack),
	}
}

// NewWithWriters creates a Printer with custom writers (for testing).
func NewWithWriters(out, err io.Writer, verbose bool) *Printer {
	return &Printer{
		out:     out,
		err:     err,
		verbose: verbose,
		green:   color.New(color.FgGreen),
		red:     color.New(color.FgRed),
		bold:    color.New(color.Bold),
		hiGreen: color.New(color.FgHiGreen),
		hiRed:   color.New(color.FgHiRed),
		yellow:  color.New(color.FgYellow),
		grey:    color.New(color.FgHiBlack),
	}
}

// Header prints a section header. In verbose mode it prints immediately.
// In quiet mode it stores the header and only prints it if a failure follows.
func (p *Printer) Header(name string) {
	if p.verbose {
		fmt.Fprintf(p.out, "%s:\n", p.bold.Sprint(name))
		return
	}
	p.pendingHeader = name
}

// flushHeader prints and clears any pending header.
func (p *Printer) flushHeader() {
	if p.pendingHeader != "" {
		fmt.Fprintf(p.out, "%s:\n", p.bold.Sprint(p.pendingHeader))
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
		p.linkOk(r.Name, r.Target)
	case link.StatusError:
		msg := "error"
		if r.Error != nil {
			msg = r.Error.Error()
		}
		p.linkFail(r.Name, msg)
	case link.StatusMissing:
		p.linkFail(r.Name, r.Message)
	case link.StatusDiff:
		p.linkFail(r.Name, r.Message)
	}
}

// PrintHookStatus prints a hook status check result.
func (p *Printer) PrintHookStatus(s hooks.HookStatus) {
	name := hooks.DisplayName(s.Name)
	if s.Ok() {
		p.hookOk(name, 0)
	} else {
		p.linkFail(name, "hook failed")
	}
}

// Summary prints a one-line run summary with ok/total counts per phase.
func (p *Printer) Summary(preOk, preTotal, linksOk, linksTotal, postOk, postTotal int) {
	failed := linksOk < linksTotal || preOk < preTotal || postOk < postTotal
	symbol := p.green.Sprint("✓")
	if failed {
		symbol = p.red.Sprint("✗")
	}
	sep := p.grey.Sprint("·")
	parts := []string{fmt.Sprintf("%s dottie", symbol)}
	if preTotal > 0 {
		parts = append(parts, fmt.Sprintf("hooks:pre %s", p.colorCount(preOk, preTotal)))
	}
	parts = append(parts, fmt.Sprintf("links %s", p.colorCount(linksOk, linksTotal)))
	if postTotal > 0 {
		parts = append(parts, fmt.Sprintf("hooks:post %s", p.colorCount(postOk, postTotal)))
	}
	fmt.Fprintln(p.out, strings.Join(parts, " "+sep+" "))
}

// PrintDottieStatus prints the dottie info section (binary, config, version, update).
func (p *Printer) PrintDottieStatus(binary, configPath, version, latest string, upToDate bool) {
	p.Header("dottie")
	fmt.Fprintf(p.out, "  binary: %s\n", binary)
	fmt.Fprintf(p.out, "  config: %s\n", configPath)
	if upToDate {
		fmt.Fprintf(p.out, "  version: %s\n", version)
	} else {
		fmt.Fprintf(p.out, "  version: %s %s\n", version, p.yellow.Sprintf("(update available: %s)", latest))
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
	fmt.Fprintf(p.out, "  %s %s (%.1fs)\n", p.green.Sprint("✓"), name, d.Seconds())
}

func (p *Printer) hookFail(name, phase string, d time.Duration, output string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  %s %s %s hook failed (%.1fs)\n", p.red.Sprint("✗"), name, phase, d.Seconds())
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			fmt.Fprintf(p.out, "    %s\n", line)
		}
	}
}

func (p *Printer) linkOk(name, target string) {
	if !p.verbose {
		return
	}
	fmt.Fprintf(p.out, "  %s %s -> %s\n", p.green.Sprint("✓"), name, target)
}

func (p *Printer) linkFail(name, msg string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  %s %s (%s)\n", p.red.Sprint("✗"), name, msg)
}

func (p *Printer) colorCount(ok, total int) string {
	s := fmt.Sprintf("%d/%d", ok, total)
	if ok < total {
		return p.hiRed.Sprint(s)
	}
	return p.hiGreen.Sprint(s)
}
