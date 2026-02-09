package cli

import (
	"flag"
	"fmt"
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
		return cmdInit(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "hooks":
		return cmdHooks(args[1:])
	case "status":
		return cmdStatus(args[1:])
	case "update":
		return cmdUpdate()
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

func cmdInit(args []string) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	if err := dotinit.Init(dir); err != nil {
		return fatal(err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fatal(err)
	}
	fmt.Printf("Initialized dotfiles repository at %s\n", absDir)

	return 0
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	force := fs.Bool("f", false, "force")
	verbose := fs.Bool("v", false, "verbose")
	veryVerbose := fs.Bool("vv", false, "show all output")
	if err := fs.Parse(args); err != nil {
		return fatal(err)
	}

	p := console.New(verbosity(*verbose, *veryVerbose))

	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	hookRunner := newHookRunner(cfg)
	// Run pre-link hooks
	preOk, preTotal := runHooksPhase(hookRunner, p, "pre-link", *dryRun)

	// Link dotfiles
	linker := link.New(cfg)
	results, err := linker.Link(*dryRun, *force)
	if err != nil {
		return fatal(err)
	}

	linksOk := 0
	p.Header("links")
	for _, r := range results {
		p.PrintLink(r)
		if r.Status != link.StatusError {
			linksOk++
		}
	}

	// Run post-link hooks
	postOk, postTotal := runHooksPhase(hookRunner, p, "post-link", *dryRun)

	p.Summary(preOk, preTotal, linksOk, len(results), postOk, postTotal)

	if linksOk < len(results) || preOk < preTotal || postOk < postTotal {
		return 1
	}
	return 0
}

func cmdHooks(args []string) int {
	if len(args) < 1 {
		printHooksUsage()
		return 1
	}

	switch args[0] {
	case "list":
		return cmdHooksList()
	case "run":
		return cmdHooksRun(args[1:])
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

func cmdHooksList() int {
	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	hookRunner := newHookRunner(cfg)

	hooksList, err := hookRunner.List()
	if err != nil {
		return fatal(err)
	}

	if len(hooksList) == 0 {
		fmt.Println("No active hooks found")
		return 0
	}

	fmt.Println("hooks:")
	for _, h := range hooksList {
		fmt.Printf("  %s\n", filepath.Base(h))
	}

	return 0
}

func cmdHooksRun(args []string) int {
	fs := flag.NewFlagSet("hooks run", flag.ContinueOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	veryVerbose := fs.Bool("vv", false, "show all output")
	if err := fs.Parse(args); err != nil {
		return fatal(err)
	}

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
		return fatal(err)
	}

	hookRunner := newHookRunner(cfg)
	p := console.New(verbosity(true, *veryVerbose))

	ok, total := runHooksPhase(hookRunner, p, phase, *dryRun)
	if ok < total {
		return 1
	}
	return 0
}

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "verbose")
	veryVerbose := fs.Bool("vv", false, "show all output")
	if err := fs.Parse(args); err != nil {
		return fatal(err)
	}

	p := console.New(verbosity(*verbose, *veryVerbose))

	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	// Start hooks check in background
	hookRunner := newHookRunner(cfg)
	hooksChan := hookRunner.StartStatusCheck()

	// Start version check in background
	versionChan := update.GetVersion(version)

	// Check dotfile status
	linker := link.New(cfg)
	results, err := linker.Check()
	if err != nil {
		return fatal(err)
	}

	var lc console.LinkCounts
	p.Header("links")
	for _, r := range results {
		r.Target = formatTargetPath(cfg, r.Target)
		p.PrintLink(r)
		switch r.Status {
		case link.StatusLinked:
			lc.Ok++
		case link.StatusMissing:
			lc.Missing++
		case link.StatusDiff:
			lc.Diff++
		case link.StatusError:
			lc.Error++
		}
	}

	// Wait for hooks and print results
	hookResult := <-hooksChan
	if hookResult.Err != nil {
		p.Errorf("Error running status hooks: %v", hookResult.Err)
		return 1
	}

	var hc console.HookCounts
	if len(hookResult.Hooks) > 0 {
		p.Header("hooks")
		for _, h := range hookResult.Hooks {
			p.PrintHookStatus(h)
			if h.Ok() {
				hc.Ok++
			} else if h.NeedsUpdate() {
				hc.Update++
			} else {
				hc.Err++
			}
		}
	}

	// Get version info
	var latest string
	upToDate := true
	select {
	case vr := <-versionChan:
		if vr.Err == nil {
			latest = vr.Latest
			upToDate = vr.UpToDate
		}
	case <-time.After(2 * time.Second):
	}

	if p.Verbosity() >= console.Verbose {
		// Verbose/Everything: show dottie info section
		binary, err := os.Executable()
		if err != nil {
			binary = "unknown"
		}
		configPath := filepath.Join(cfg.RepoRoot(), cfg.File())
		p.PrintDottieStatus(binary, configPath, version, latest, upToDate)
	} else {
		// Quiet: compact summary
		p.StatusSummary(lc, hc, version, latest, upToDate)
	}

	allOk := lc.Missing == 0 && lc.Diff == 0 && lc.Error == 0
	hooksOk := hc.Err == 0
	if !allOk || !hooksOk {
		return 1
	}
	return 0
}

// formatTargetPath replaces the home directory prefix with ~ for display.
func formatTargetPath(cfg *config.Config, path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if cfg.TargetDir == home && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func cmdUpdate() int {
	if err := update.Install(version); err != nil {
		return fatal(err)
	}
	return 0
}

func runHooksPhase(runner *hooks.Runner, p *console.Printer, phase string, dryRun bool) (ok, total int) {
	p.Header("hooks " + phase)
	for r := range runner.RunPhase(phase, dryRun) {
		p.PrintHook(r, phase)
		total++
		if r.Ok() {
			ok++
		}
	}
	return ok, total
}

func newHookRunner(cfg *config.Config) *hooks.Runner {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return hooks.New(cfg.GetHooksPath(), hooks.EnvVars{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})
}

func verbosity(v, vv bool) console.Verbosity {
	if vv {
		return console.Everything
	}
	if v {
		return console.Verbose
	}
	return console.Quiet
}

func fatal(err error) int {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return 1
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
