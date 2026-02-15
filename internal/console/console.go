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
	"github.com/mattn/go-isatty"
)

// Verbosity controls output detail level.
type Verbosity int

const (
	// Quiet shows only failures.
	Quiet Verbosity = iota
	// Verbose shows headers and ok results.
	Verbose
	// Everything shows all output including hook stdout/stderr.
	Everything
)

// Printer handles all CLI output formatting.
// In verbose mode, headers print immediately and ok results are shown.
// In quiet mode, headers are lazy (only printed when a failure follows)
// and ok results are suppressed.
type Printer struct {
	out           io.Writer
	err           io.Writer
	verbosity     Verbosity
	targetDir     string
	pendingHeader string
	seen          map[string]bool
	green         *color.Color
	red           *color.Color
	bold          *color.Color
	hiGreen       *color.Color
	hiRed         *color.Color
	yellow        *color.Color
	grey          *color.Color
}

// New creates a Printer that writes to stdout/stderr.
func New(v Verbosity) *Printer {
	return &Printer{
		out:       os.Stdout,
		err:       os.Stderr,
		verbosity: v,
		green:     color.New(color.FgGreen),
		red:       color.New(color.FgRed),
		bold:      color.New(color.Bold),
		hiGreen:   color.New(color.FgHiGreen),
		hiRed:     color.New(color.FgHiRed),
		yellow:    color.New(color.FgYellow),
		grey:      color.New(color.FgHiBlack),
	}
}

// NewWithWriters creates a Printer with custom writers (for testing).
func NewWithWriters(out, err io.Writer, v Verbosity) *Printer {
	return &Printer{
		out:       out,
		err:       err,
		verbosity: v,
		green:     color.New(color.FgGreen),
		red:       color.New(color.FgRed),
		bold:      color.New(color.Bold),
		hiGreen:   color.New(color.FgHiGreen),
		hiRed:     color.New(color.FgHiRed),
		yellow:    color.New(color.FgYellow),
		grey:      color.New(color.FgHiBlack),
	}
}

// Verbosity returns the printer's verbosity level.
func (p *Printer) Verbosity() Verbosity { return p.verbosity }

// Out returns the printer's output writer.
func (p *Printer) Out() io.Writer { return p.out }

// SetTargetDir sets the target directory used for display path formatting.
func (p *Printer) SetTargetDir(dir string) { p.targetDir = dir }

// IsTTY returns true if the output writer is a terminal.
func (p *Printer) IsTTY() bool {
	if f, ok := p.out.(*os.File); ok {
		return isatty.IsTerminal(f.Fd())
	}
	return false
}

// Header prints a section header. In verbose mode it prints immediately.
// In quiet mode it stores the header and only prints it if a failure follows.
func (p *Printer) Header(name string) {
	if p.verbosity >= Verbose {
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

// sectionHeader prints a section header once per unique name.
func (p *Printer) sectionHeader(name string) {
	if p.seen[name] {
		return
	}
	if p.seen == nil {
		p.seen = make(map[string]bool)
	}
	p.seen[name] = true
	p.Header(name)
}

// PrintHook prints a hook result, auto-emitting the section header on first call per phase.
func (p *Printer) PrintHook(r hooks.Result, phase string) {
	p.sectionHeader("hooks " + phase)
	if r.Ok() {
		p.hookOk(r.Name, r.Elapsed, r.Output)
	} else if phase == "status" && r.ExitCode == 1 {
		p.hookNeedsUpdate(r.Name, r.Output)
	} else {
		p.hookFail(r.Name, phase, r.Elapsed, r.Output)
	}
}

// PrintLink prints a link result, auto-emitting the "links" section header on first call.
func (p *Printer) PrintLink(r link.Result) {
	p.sectionHeader("links")
	target := p.formatTarget(r.Target)
	switch r.Status {
	case link.StatusLinked, link.StatusAlreadyLinked, link.StatusWouldLink:
		p.linkOk(r.Name, target)
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
	case link.StatusDangling:
		p.linkInfo(r.Name, "orphan")
	}
}

// Summary prints a one-line run summary with ok/total counts per phase.
func (p *Printer) Summary(preOk, preTotal int, ls link.Summary, postOk, postTotal int) {
	failed := ls.Errors > 0 || preOk < preTotal || postOk < postTotal
	symbol := p.green.Sprint("✓")
	if failed {
		symbol = p.red.Sprint("✗")
	}
	sep := p.grey.Sprint("·")
	parts := []string{fmt.Sprintf("%s %s", symbol, p.bold.Sprint("dottie is done"))}
	if preTotal > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", p.grey.Sprint("hooks:pre"), p.colorCount(preOk, preTotal)))
	}
	parts = append(parts, fmt.Sprintf("%s %s", p.grey.Sprint("links"), p.linkSummary(ls)))
	if postTotal > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", p.grey.Sprint("hooks:post"), p.colorCount(postOk, postTotal)))
	}
	fmt.Fprintln(p.out, strings.Join(parts, " "+sep+" "))
}

func (p *Printer) linkSummary(ls link.Summary) string {
	parts := []string{p.grey.Sprintf("%d", ls.Existing)}
	if ls.Added > 0 {
		parts = append(parts, p.green.Sprintf("+%d", ls.Added))
	}
	if ls.Pruned > 0 {
		parts = append(parts, p.red.Sprintf("-%d", ls.Pruned))
	}
	if ls.Errors > 0 {
		parts = append(parts, p.red.Sprintf("!%d", ls.Errors))
	}
	return strings.Join(parts, " ")
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

// LinkCounts holds link status counts for the compact summary.
type LinkCounts struct {
	Ok, Missing, Diff, Dangling, Error int
}

// HookCounts holds hook status counts for the compact summary.
type HookCounts struct {
	Ok, Update, Err int
}

// total returns the total number of hooks.
func (hc HookCounts) total() int {
	return hc.Ok + hc.Update + hc.Err
}

// StatusSummary prints a compact 3-line status summary.
func (p *Printer) StatusSummary(lc LinkCounts, hc HookCounts, version, latest string, upToDate bool) {
	sep := p.grey.Sprint("·")

	// Line 1: links
	parts := []string{p.green.Sprintf("ok:%d", lc.Ok)}
	if lc.Missing > 0 {
		parts = append(parts, p.red.Sprintf("unlinked:%d", lc.Missing))
	}
	if lc.Diff > 0 {
		parts = append(parts, p.red.Sprintf("diff:%d", lc.Diff))
	}
	if lc.Dangling > 0 {
		parts = append(parts, p.red.Sprintf("orphan:%d", lc.Dangling))
	}
	if lc.Error > 0 {
		parts = append(parts, p.red.Sprintf("err:%d", lc.Error))
	}
	fmt.Fprintf(p.out, "links: %s\n", strings.Join(parts, " "+sep+" "))

	// Line 2: hooks (skip if no hooks)
	if hc.total() > 0 {
		parts = []string{p.green.Sprintf("ok:%d", hc.Ok)}
		if hc.Update > 0 {
			parts = append(parts, p.yellow.Sprintf("update:%d", hc.Update))
		}
		if hc.Err > 0 {
			parts = append(parts, p.red.Sprintf("err:%d", hc.Err))
		}
		fmt.Fprintf(p.out, "hooks: %s\n", strings.Join(parts, " "+sep+" "))
	}

	// Line 3: dottie version
	if upToDate {
		fmt.Fprintf(p.out, "dottie %s\n", version)
	} else {
		fmt.Fprintf(p.out, "dottie %s %s\n", version, p.yellow.Sprintf("(update available: %s)", latest))
	}
}

// Errorf prints an error message to stderr. Always prints.
func (p *Printer) Errorf(format string, args ...any) {
	fmt.Fprintf(p.err, format+"\n", args...)
}

func (p *Printer) hookOk(name string, d time.Duration, output string) {
	if p.verbosity < Verbose {
		return
	}
	fmt.Fprintf(p.out, "  %s %s (%.1fs)\n", p.green.Sprint("✓"), name, d.Seconds())
	if p.verbosity >= Everything {
		p.printOutputLines(output)
	}
}

func (p *Printer) hookNeedsUpdate(name, output string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  %s %s (needs update)\n", p.yellow.Sprint("~"), name)
	if p.verbosity >= Everything {
		p.printOutputLines(output)
	}
}

func (p *Printer) printOutputLines(output string) {
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			fmt.Fprintf(p.out, "    %s %s\n", p.grey.Sprint("|"), p.grey.Sprint(line))
		}
	}
}

func (p *Printer) hookFail(name, phase string, d time.Duration, output string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  %s %s %s hook failed (%.1fs)\n", p.red.Sprint("✗"), name, phase, d.Seconds())
	p.printOutputLines(output)
}

func (p *Printer) linkOk(name, target string) {
	if p.verbosity < Verbose {
		return
	}
	fmt.Fprintf(p.out, "  %s %s -> %s\n", p.green.Sprint("✓"), name, target)
}

func (p *Printer) linkInfo(name, msg string) {
	if p.verbosity < Verbose {
		return
	}
	fmt.Fprintf(p.out, "  %s %s (%s)\n", p.red.Sprint("✗"), name, msg)
}

func (p *Printer) linkFail(name, msg string) {
	p.flushHeader()
	fmt.Fprintf(p.out, "  %s %s (%s)\n", p.red.Sprint("✗"), name, msg)
}

// formatTarget replaces the home directory prefix with ~ for display.
func (p *Printer) formatTarget(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if p.targetDir == home && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func (p *Printer) colorCount(ok, total int) string {
	if ok == total {
		return p.grey.Sprintf("%d", total)
	}
	return p.hiRed.Sprintf("%d/%d", ok, total)
}
