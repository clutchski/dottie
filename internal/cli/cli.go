package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/console"
	"github.com/clutchski/dottie/internal/hooks"
	dotinit "github.com/clutchski/dottie/internal/init"
	"github.com/clutchski/dottie/internal/link"
	"github.com/clutchski/dottie/internal/status"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// ANSI color codes
const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
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
	hookRunner := hooks.New(cfg.GetHooksPath(), cwd, cfg.GetTargetDir())

	// Track per-hook results across phases (ok only if all phases pass)
	hookOk := make(map[string]bool)

	// Run pre-link hooks
	preHooks, err := runHookPhase(hookRunner, "pre-link", *dryRun, *verbose, con)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running pre-link hooks: %v\n", err)
		return 1
	}
	for _, h := range preHooks {
		hookOk[h.Name] = h.Ok
	}

	// Link dotfiles
	linker := link.New(cfg)
	results, err := linker.Link(*dryRun, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Print results based on verbosity
	if *verbose {
		fmt.Fprintln(con.Stdout, "dotfiles:")
	}
	for _, r := range results {
		if r.BackupPath != "" {
			fmt.Fprintf(con.Stdout, "backed up %s -> %s\n", r.Target, r.BackupPath)
		}

		if *verbose {
			var symbol, color string
			switch r.Status {
			case link.StatusLinked:
				symbol = "+"
				color = colorGreen
			case link.StatusWouldLink:
				symbol = "~"
				color = colorGreen
			case link.StatusAlreadyLinked:
				symbol = "✓"
				color = colorGreen
			case link.StatusSkipped:
				symbol = "-"
				color = colorRed
			case link.StatusError:
				symbol = "x"
				color = colorRed
			}
			fmt.Fprintf(con.Stdout, "  %s%s%s %s -> %s\n", color, symbol, colorReset, filepath.Base(r.Source), r.Target)
			if r.Error != nil {
				fmt.Fprintf(con.Stderr, "    %v\n", r.Error)
			}
		}
	}

	// Run post-link hooks
	postHooks, err := runHookPhase(hookRunner, "post-link", *dryRun, *verbose, con)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running post-link hooks: %v\n", err)
		return 1
	}
	for _, h := range postHooks {
		if !h.Ok {
			hookOk[h.Name] = false
		}
	}

	// Default mode: print summary
	if !*verbose && !*quiet {
		printRunSummary(con.Stdout, results)
		// Build deduplicated hook statuses preserving order from preHooks
		var dedupedHooks []hooks.HookStatus
		for _, h := range preHooks {
			dedupedHooks = append(dedupedHooks, hooks.HookStatus{Name: h.Name, Ok: hookOk[h.Name]})
		}
		printHooksSummary(con.Stdout, dedupedHooks)
	}

	return 0
}

// runHookPhase runs hooks for a phase. In verbose mode, output streams through.
// In default mode, output is captured and only shown if the hook fails.
// Returns per-hook statuses.
func runHookPhase(runner *hooks.Runner, phase string, dryRun, verbose bool, con *console.Console) ([]hooks.HookStatus, error) {
	if verbose {
		err := runner.Run(phase, dryRun, verbose, con)
		return nil, err
	}
	// Default/quiet: capture output, show on failure
	var bufOut, bufErr bytes.Buffer
	bufCon := &console.Console{Stdout: &bufOut, Stderr: &bufErr}
	statuses, err := runner.RunAll(phase, dryRun, false, bufCon)
	if err != nil {
		return nil, err
	}
	// Show captured output for any failed hooks
	for _, s := range statuses {
		if !s.Ok {
			_, _ = con.Stdout.Write(bufOut.Bytes())
			_, _ = con.Stderr.Write(bufErr.Bytes())
			break
		}
	}
	return statuses, nil
}

// printRunSummary prints the dotfiles summary line for a run.
func printRunSummary(w io.Writer, results []link.Result) {
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
	hookRunner := hooks.New(cfg.GetHooksPath(), cwd, cfg.GetTargetDir())

	hooksList, err := hookRunner.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(hooksList) == 0 {
		fmt.Println("No active hooks found")
		return 0
	}

	fmt.Println("Active hooks:")
	for _, h := range hooksList {
		fmt.Printf("  %s\n", filepath.Base(h))
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
	hookRunner := hooks.New(cfg.GetHooksPath(), cwd, cfg.GetTargetDir())

	if err := hookRunner.Run(phase, *dryRun, *verbose, console.New(false)); err != nil {
		fmt.Fprintf(os.Stderr, "Error running %s hooks: %v\n", phase, err)
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

	fmt.Fprintf(con.Stdout, "dottie %s\n", version)

	// Start hooks first (they're slower)
	cwd, _ := os.Getwd()
	hookRunner := hooks.New(cfg.GetHooksPath(), cwd, cfg.GetTargetDir())
	hooksChan := hookRunner.StartStatusCheck()

	// Print dotfiles status
	checker := status.New(cfg)
	var allOk bool
	if *quiet {
		allOk, _, err = checker.Check()
	} else if *verbose {
		allOk, err = checker.PrintVerbose(con.Stdout)
	} else {
		allOk, err = checker.PrintSummary(con.Stdout)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Wait for hooks and print results
	hookResult := <-hooksChan
	if hookResult.Err != nil {
		fmt.Fprintf(os.Stderr, "Error running status hooks: %v\n", hookResult.Err)
		return 1
	}

	hooksOk := printHooksSummary(con.Stdout, hookResult.Hooks)

	if !allOk || !hooksOk {
		return 1
	}
	return 0
}

// printHooksSummary prints [ok]/[x] lines for hooks. Returns true if all ok.
func printHooksSummary(w io.Writer, hookStatuses []hooks.HookStatus) bool {
	if len(hookStatuses) == 0 {
		return true
	}

	var okNames, failNames []string
	for _, h := range hookStatuses {
		name := hooks.DisplayName(h.Name)
		if h.Ok {
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
