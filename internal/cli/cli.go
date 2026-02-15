package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/console"
	"github.com/clutchski/dottie/internal/hooks"
	dotinit "github.com/clutchski/dottie/internal/init"
	"github.com/clutchski/dottie/internal/link"
	"github.com/clutchski/dottie/internal/run"
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
  status         Show status of dotfiles
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
	noProgress := fs.Bool("no-progress", false, "disable progress spinner")
	if err := fs.Parse(args); err != nil {
		return fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	p := console.New(verbosity(*verbose, *veryVerbose))
	p.SetTargetDir(cfg.TargetDir)
	showProgress := p.IsTTY() && p.Verbosity() == console.Quiet && !*noProgress

	runner := run.New(cfg, nil, *dryRun, *force)
	events := runner.Start()
	spinner := run.NewSpinner(p.Out(), showProgress)

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				ticker.Stop()
				spinner.Clear()
				result := runner.Wait()
				p.Summary(result.PreOk, result.PreTotal, result.Links,
					result.PostOk, result.PostTotal)
				return result.ExitCode
			}
			spinner.Clear()
			switch e := ev.(type) {
			case run.HookEvent:
				p.PrintHook(e.Result, e.Phase)
			case run.LinkEvent:
				p.PrintLink(e.Result)
			}
			spinner.Render(runner.Phase(), runner.ActiveHooks())
		case <-ticker.C:
			spinner.Render(runner.Phase(), runner.ActiveHooks())
		}
	}
}

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	verbose := fs.Bool("v", false, "verbose")
	veryVerbose := fs.Bool("vv", false, "show all output")
	if err := fs.Parse(args); err != nil {
		return fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	p := console.New(verbosity(*verbose, *veryVerbose))
	p.SetTargetDir(cfg.TargetDir)

	// Start hooks check in background
	hooksChan := newHookRunner(cfg).RunStatusAsync()

	// Start version check in background
	versionChan := update.GetVersion(version)

	// Check dotfile status (includes dangling links from manifest)
	linker := link.New(cfg)
	results, err := linker.Check()
	if err != nil {
		return fatal(err)
	}

	var lc console.LinkCounts
	for _, r := range results {
		p.PrintLink(r)
		switch r.Status {
		case link.StatusLinked:
			lc.Ok++
		case link.StatusMissing:
			lc.Missing++
		case link.StatusDiff:
			lc.Diff++
		case link.StatusDangling:
			lc.Dangling++
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
	for _, h := range hookResult.Results {
		p.PrintHook(h, "status")
		switch h.Status() {
		case hooks.StatusOk:
			hc.Ok++
		case hooks.StatusNeedsUpdate:
			hc.Update++
		case hooks.StatusFailed:
			hc.Err++
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

	allOk := lc.Missing == 0 && lc.Diff == 0 && lc.Dangling == 0 && lc.Error == 0
	hooksOk := hc.Err == 0
	if !allOk || !hooksOk {
		return 1
	}
	return 0
}

func cmdUpdate() int {
	if err := update.Install(version); err != nil {
		return fatal(err)
	}
	return 0
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
