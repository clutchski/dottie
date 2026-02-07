package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/console"
	"github.com/clutchski/dottie/internal/hooks"
	dotinit "github.com/clutchski/dottie/internal/init"
	"github.com/clutchski/dottie/internal/link"
	"github.com/clutchski/dottie/internal/update"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

// SetVersion sets version info (called from main).
func SetVersion(v, c, d string) {
	version = v
	commit = c
	date = d
}

// Run executes the CLI.
func Run(args []string) int {
	if len(args) < 1 {
		printUsage()
		return 1
	}

	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "run":
		return runRun(args[1:])
	case "hooks":
		return runHooks(args[1:])
	case "status":
		return runStatus(args[1:])
	case "update":
		return runUpdate()
	case "version", "--version", "-v":
		printVersion()
		return 0
	case "help", "--help", "-h":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Println(`dottie - Simple dotfiles manager

Usage:
  dottie <command> [options]

Commands:
  init [dir]     Initialize a new dotfiles repository
  run            Run hooks and symlink dotfiles to home directory
  hooks          Manage hooks (list, run)
  status         Show status of dotfiles
  update         Update dottie to the latest version
  version        Show version information
  help           Show this help message`)
}

func printVersion() {
	fmt.Printf("dottie %s\n", version)
	fmt.Printf("  commit: %s\n", commit)
	fmt.Printf("  built:  %s\n", date)
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	_ = fs.Parse(args)

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	if err := dotinit.Init(dir, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if !*dryRun {
		absDir, _ := filepath.Abs(dir)
		fmt.Printf("Initialized dotfiles repository at %s\n", absDir)
	}

	return 0
}

func runRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	force := fs.Bool("f", false, "force")
	verbose := fs.Bool("v", false, "verbose")
	quiet := fs.Bool("q", false, "quiet")
	_ = fs.Parse(args)

	con := console.New(*quiet)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	cwd, _ := os.Getwd()
	hooks := hooks.New(cfg.GetHooksPath(), map[string]string{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})

	// Run pre-link hooks
	preEvents, err := hooks.RunPreLink(*dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running pre-link hooks: %v\n", err)
		return 1
	}
	if *verbose {
		fmt.Fprintln(con.Stdout, "hooks:")
	}
	preDone := processRunHooks(preEvents, *verbose, con)

	// Link dotfiles
	linker := link.New(cfg)
	linkEvents, err := linker.Run(*dryRun, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if *verbose {
		fmt.Fprintln(con.Stdout, "dotfiles:")
	}
	var results []link.Event
	for r := range linkEvents {
		results = append(results, r)

		if r.BackupPath != "" {
			fmt.Fprintf(con.Stdout, "backed up %s -> %s\n", r.Target, r.BackupPath)
		}

		if *verbose {
			symbol, color := linkStatusSymbol(r.Status)
			fmt.Fprintf(con.Stdout, "  %s%s%s %s -> %s\n", color, symbol, colorReset, filepath.Base(r.Source), r.Target)
			if r.Error != nil {
				fmt.Fprintf(con.Stderr, "    %v\n", r.Error)
			}
		}
	}

	// Run post-link hooks
	postEvents, err := hooks.RunPostLink(*dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running post-link hooks: %v\n", err)
		return 1
	}
	postDone := processRunHooks(postEvents, *verbose, con)

	// Default mode: print summary
	if !*verbose && !*quiet {
		printRunSummary(con.Stdout, results)

		// Merge hook results across phases (ok only if all phases pass)
		hookOk := make(map[string]bool)
		for _, ev := range preDone {
			hookOk[ev.Hook] = ev.Err == nil
		}
		for _, ev := range postDone {
			if _, seen := hookOk[ev.Hook]; !seen {
				hookOk[ev.Hook] = ev.Err == nil
			} else if ev.Err != nil {
				hookOk[ev.Hook] = false
			}
		}

		printHooksSummary(con.Stdout, uniqueHookEvents(preDone, postDone), hookOk)
	}

	return 0
}

// processRunHooks drains hook events for the `run` command.
// In verbose mode, prints start/done lines and streams failed output.
// In non-verbose mode, prints stderr from failed hooks.
// Returns Done events for summary building.
func processRunHooks(events <-chan hooks.Event, verbose bool, con *console.Console) []hooks.Event {
	var done []hooks.Event
	for ev := range events {
		switch ev.Kind {
		case hooks.Started:
			if verbose {
				fmt.Fprintf(con.Stdout, "  %s◎%s start: %s\n", colorYellow, colorReset, ev.HookDisplay())
			}
		case hooks.Done:
			done = append(done, ev)
			if verbose {
				name := ev.HookDisplay()
				if ev.Err != nil {
					fmt.Fprintf(con.Stdout, "  %sx%s done:  %s (%.1fs)\n", colorRed, colorReset, name, ev.Duration.Seconds())
					_, _ = con.Stdout.Write(ev.Stdout)
					_, _ = con.Stderr.Write(ev.Stderr)
				} else {
					fmt.Fprintf(con.Stdout, "  %s✓%s done:  %s (%.1fs)\n", colorGreen, colorReset, name, ev.Duration.Seconds())
				}
			} else if ev.Err != nil {
				_, _ = con.Stderr.Write(ev.Stderr)
			}
		}
	}
	return done
}

// printRunSummary prints the dotfiles summary line for a run.
func printRunSummary(w io.Writer, results []link.Event) {
	added := 0
	linked := 0
	var errorNames []string
	for _, r := range results {
		switch r.Status {
		case link.StatusLinked:
			added++
		case link.StatusAlreadyLinked:
			linked++
		case link.StatusError:
			errorNames = append(errorNames, filepath.Base(r.Source))
		}
	}

	fmt.Fprintln(w, "dotfiles:")
	if added > 0 || linked > 0 {
		var parts []string
		if added > 0 {
			parts = append(parts, fmt.Sprintf("added:%d", added))
		}
		if linked > 0 {
			parts = append(parts, fmt.Sprintf("linked:%d", linked))
		}
		fmt.Fprintf(w, "  %s✓%s %s\n", colorGreen, colorReset, strings.Join(parts, " "))
	}
	if len(errorNames) > 0 {
		fmt.Fprintf(w, "  %sx%s %s\n", colorRed, colorReset, strings.Join(errorNames, ", "))
	}
}

func runHooks(args []string) int {
	if len(args) < 1 {
		printHooksUsage()
		return 1
	}

	switch args[0] {
	case "list":
		return runHooksList(args[1:])
	case "run":
		return runHooksRun(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown hooks subcommand: %s\n", args[0])
		printHooksUsage()
		return 1
	}
}

func printHooksUsage() {
	fmt.Println(`Usage: dottie hooks <subcommand>

Subcommands:
  list           List active hooks
  run <phase>    Run hooks for a phase (pre-link, post-link, status)

Examples:
  dottie hooks list
  dottie hooks run pre-link
  dottie hooks run status`)
}

func runHooksList(args []string) int {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	cwd, _ := os.Getwd()
	hooks := hooks.New(cfg.GetHooksPath(), map[string]string{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})

	list, err := hooks.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(list) == 0 {
		fmt.Println("No active hooks found")
		return 0
	}

	fmt.Println("Active hooks:")
	for _, h := range list {
		fmt.Printf("  %s\n", hookDisplayName(h))
	}

	return 0
}

func runHooksRun(args []string) int {
	fs := flag.NewFlagSet("hooks run", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	verbose := fs.Bool("v", false, "verbose")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: phase argument required (pre-link, post-link, status)")
		return 1
	}

	phase := fs.Arg(0)
	if phase != "pre-link" && phase != "post-link" && phase != "status" {
		fmt.Fprintf(os.Stderr, "Error: invalid phase %q (must be pre-link, post-link, or status)\n", phase)
		return 1
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	cwd, _ := os.Getwd()
	hooks := hooks.New(cfg.GetHooksPath(), map[string]string{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})

	events, err := runPhase(hooks, phase, *dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running %s hooks: %v\n", phase, err)
		return 1
	}

	return processHooksCommand(events, phase, *verbose)
}

// runPhase calls the appropriate phase method on the runner.
func runPhase(runner *hooks.Runner, phase string, dryRun bool) (<-chan hooks.Event, error) {
	switch phase {
	case "pre-link":
		return runner.RunPreLink(dryRun)
	case "post-link":
		return runner.RunPostLink(dryRun)
	case "status":
		return runner.CheckStatus()
	default:
		return nil, fmt.Errorf("invalid phase: %s", phase)
	}
}

// processHooksCommand drains hook events for `dottie hooks run`.
// Prints start/done lines in verbose mode. Reports first failure and returns exit code.
func processHooksCommand(events <-chan hooks.Event, phase string, verbose bool) int {
	var failed []hooks.Event
	for ev := range events {
		switch ev.Kind {
		case hooks.Started:
			if verbose {
				fmt.Printf("  %s◎%s start: %s\n", colorYellow, colorReset, ev.HookDisplay())
			}
		case hooks.Done:
			if verbose {
				name := ev.HookDisplay()
				if ev.Err != nil {
					fmt.Printf("  %sx%s done:  %s (%.1fs)\n", colorRed, colorReset, name, ev.Duration.Seconds())
				} else {
					fmt.Printf("  %s✓%s done:  %s (%.1fs)\n", colorGreen, colorReset, name, ev.Duration.Seconds())
				}
			}
			if ev.Err != nil {
				failed = append(failed, ev)
			}
		}
	}

	for _, ev := range failed {
		_, _ = os.Stdout.Write(ev.Stdout)
		_, _ = os.Stderr.Write(ev.Stderr)
	}

	if len(failed) > 0 {
		return 1
	}
	return 0
}

func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose")
	quiet := fs.Bool("q", false, "quiet")
	_ = fs.Parse(args)

	con := console.New(*quiet)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(con.Stdout, "dottie %s\n", version)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Start version check in background
	versionChan := update.GetVersion(version)

	// Start hooks check in background
	cwd, _ := os.Getwd()
	hooks := hooks.New(cfg.GetHooksPath(), map[string]string{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})
	hookEvents, err := hooks.CheckStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running status hooks: %v\n", err)
		return 1
	}

	// Get dotfiles status while hooks run in background
	linker := link.New(cfg)
	statuses, err := linker.CheckStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	allOk := true
	for _, s := range statuses {
		if s.Status != link.FileStatusLinked {
			allOk = false
			break
		}
	}

	if !*quiet {
		if *verbose {
			printStatusVerbose(con.Stdout, statuses, cfg)
		} else {
			printStatusSummary(con.Stdout, statuses)
		}
	}

	// Wait for hooks and print summary
	hooksOk := processStatusHooks(hookEvents, con.Stdout)

	// Show dottie version and update status
	fmt.Printf("dottie: %s", version)
	select {
	case vr := <-versionChan:
		if vr.Err == nil {
			if vr.UpToDate {
				fmt.Printf(" %s(up to date)%s", colorGreen, colorReset)
			} else {
				fmt.Printf(" %s(update available: %s)%s", colorYellow, vr.Latest, colorReset)
			}
		}
	case <-time.After(2 * time.Second):
	}
	fmt.Println()

	if !allOk || !hooksOk {
		return 1
	}
	return 0
}

// processStatusHooks drains hook events for the `status` command.
// Prints the hooks summary and returns true if all hooks passed.
func processStatusHooks(events <-chan hooks.Event, w io.Writer) bool {
	var done []hooks.Event
	for ev := range events {
		if ev.Kind == hooks.Done {
			done = append(done, ev)
		}
	}

	if len(done) == 0 {
		return true
	}

	hookOk := make(map[string]bool)
	for _, ev := range done {
		hookOk[ev.Hook] = ev.Err == nil
	}
	return printHooksSummary(w, done, hookOk)
}

// printHooksSummary prints [ok]/[x] lines for hooks. Returns true if all ok.
// doneEvents are the Done events from one phase (used for ordering).
// hookOk maps hook path -> final ok status across all phases.
func printHooksSummary(w io.Writer, doneEvents []hooks.Event, hookOk map[string]bool) bool {
	if len(doneEvents) == 0 {
		return true
	}

	var okNames, failNames []string
	for _, ev := range doneEvents {
		name := ev.HookDisplay()
		if hookOk[ev.Hook] {
			okNames = append(okNames, name)
		} else {
			failNames = append(failNames, name)
		}
	}

	fmt.Fprintln(w, "hooks:")
	if len(okNames) > 0 {
		fmt.Fprintf(w, "  %s✓%s %s\n", colorGreen, colorReset, strings.Join(okNames, ", "))
	}
	if len(failNames) > 0 {
		fmt.Fprintf(w, "  %sx%s %s\n", colorRed, colorReset, strings.Join(failNames, ", "))
	}

	return len(failNames) == 0
}

// uniqueHookEvents combines done events from multiple phases, deduplicating by hook path.
func uniqueHookEvents(phases ...[]hooks.Event) []hooks.Event {
	seen := make(map[string]bool)
	var all []hooks.Event
	for _, phase := range phases {
		for _, ev := range phase {
			if !seen[ev.Hook] {
				seen[ev.Hook] = true
				all = append(all, ev)
			}
		}
	}
	return all
}

// linkStatusSymbol returns the symbol and color for a link status.
func linkStatusSymbol(s link.Status) (symbol, color string) {
	switch s {
	case link.StatusLinked:
		return "+", colorGreen
	case link.StatusWouldLink:
		return "~", colorGreen
	case link.StatusAlreadyLinked:
		return "✓", colorGreen
	case link.StatusSkipped:
		return "-", colorRed
	case link.StatusError:
		return "x", colorRed
	default:
		return "?", colorReset
	}
}

// printStatusVerbose prints the full per-file status listing.
func printStatusVerbose(w io.Writer, statuses []link.FileInfo, cfg *config.Config) {
	if len(statuses) == 0 {
		return
	}

	maxWidth := 0
	for _, s := range statuses {
		displayTarget := formatTargetPath(s.TargetPath, cfg)
		if len(displayTarget) > maxWidth {
			maxWidth = len(displayTarget)
		}
	}

	fmt.Fprintln(w, "Dotfiles:")
	for _, s := range statuses {
		var symbol, color, reason string
		switch s.Status {
		case link.FileStatusLinked:
			symbol = "✓"
			color = colorGreen
		case link.FileStatusMissing:
			symbol = "x"
			color = colorRed
			reason = "(not linked)"
		case link.FileStatusDiff:
			symbol = "!"
			color = colorYellow
			reason = "(" + s.Message + ")"
		}

		displayTarget := formatTargetPath(s.TargetPath, cfg)
		displaySource := formatSourcePath(s.SourcePath)

		if reason != "" {
			fmt.Fprintf(w, "  %s%s%s %-*s  %s%s%s\n", color, symbol, colorReset, maxWidth, displayTarget, color, reason, colorReset)
		} else {
			fmt.Fprintf(w, "  %s%s%s %-*s -> %s\n", color, symbol, colorReset, maxWidth, displayTarget, displaySource)
		}
	}
}

// printStatusSummary prints a compact status using checkmark/x format.
func printStatusSummary(w io.Writer, statuses []link.FileInfo) {
	if len(statuses) == 0 {
		return
	}

	okCount := 0
	var problemNames []string
	for _, s := range statuses {
		if s.Status == link.FileStatusLinked {
			okCount++
		} else {
			problemNames = append(problemNames, s.Name)
		}
	}

	fmt.Fprintln(w, "dotfiles:")
	if okCount > 0 {
		fmt.Fprintf(w, "  %s✓%s %d ok\n", colorGreen, colorReset, okCount)
	}
	for _, name := range problemNames {
		fmt.Fprintf(w, "  %sx%s %s\n", colorRed, colorReset, name)
	}
}

// formatTargetPath formats the target path for display (replaces HOME with ~ only if target is HOME).
func formatTargetPath(path string, cfg *config.Config) string {
	home, _ := os.UserHomeDir()
	targetDir := cfg.TargetDir

	if targetDir == home && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// formatSourcePath formats the source path for display (relative to repo root).
func formatSourcePath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return rel
}

// hookDisplayName returns a display name from a hook path.
// Strips directory and extension: "/path/to/01-homebrew.sh" -> "01-homebrew".
func hookDisplayName(path string) string {
	name := filepath.Base(path)
	if ext := filepath.Ext(name); ext != name {
		return strings.TrimSuffix(name, ext)
	}
	return name
}

func runUpdate() int {
	if err := update.Install(version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func loadConfig() (*config.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if !config.IsDottieDir(cwd) {
		return nil, fmt.Errorf("not a dottie directory; run 'dottie init' to create one")
	}

	return config.Load(cwd)
}
