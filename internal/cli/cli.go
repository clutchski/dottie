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
	_ = fs.Parse(args)

	p := console.New(*verbose)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	cwd, _ := os.Getwd()
	hookRunner := hooks.New(cfg.GetHooksPath(), hooks.EnvVars{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})
	failed := false

	// Run pre-link hooks
	p.Header("hooks pre-link")
	for r := range hookRunner.RunPhase("pre-link", *dryRun) {
		if r.Ok() {
			p.HookOk(r.Name, r.Elapsed)
		} else {
			p.HookFail(r.Name, "pre-link", r.Elapsed, r.Output)
			failed = true
		}
	}

	// Link dotfiles
	linker := link.New(cfg)
	results, err := linker.Link(*dryRun, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	p.Header("dotfiles")
	for _, r := range results {
		switch r.Status {
		case link.StatusLinked, link.StatusAlreadyLinked, link.StatusWouldLink:
			p.DotfileOk(r.Name, r.Target)
		case link.StatusError:
			msg := "error"
			if r.Error != nil {
				msg = r.Error.Error()
			}
			p.DotfileFail(r.Name, msg)
			failed = true
		case link.StatusSkipped:
			p.DotfileFail(r.Name, "skipped")
			failed = true
		}
	}

	// Run post-link hooks
	p.Header("hooks post-link")
	for r := range hookRunner.RunPhase("post-link", *dryRun) {
		if r.Ok() {
			p.HookOk(r.Name, r.Elapsed)
		} else {
			p.HookFail(r.Name, "post-link", r.Elapsed, r.Output)
			failed = true
		}
	}

	if failed {
		return 1
	}
	return 0
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
	hookRunner := hooks.New(cfg.GetHooksPath(), hooks.EnvVars{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})

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
	hookRunner := hooks.New(cfg.GetHooksPath(), hooks.EnvVars{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})
	p := console.New(*verbose)

	failed := false
	p.Header("hooks " + phase)
	for r := range hookRunner.RunPhase(phase, *dryRun) {
		if r.Ok() {
			p.HookOk(r.Name, r.Elapsed)
		} else {
			p.HookFail(r.Name, phase, r.Elapsed, r.Output)
			failed = true
		}
	}

	if failed {
		return 1
	}
	return 0
}

func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose")
	_ = fs.Parse(args)

	p := console.New(*verbose)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Start hooks check in background
	cwd, _ := os.Getwd()
	hookRunner := hooks.New(cfg.GetHooksPath(), hooks.EnvVars{
		"DOTTIE_ROOT": cwd,
		"DOTTIE_HOME": cfg.GetTargetDir(),
	})
	hooksChan := hookRunner.StartStatusCheck()

	// Start version check in background (verbose only)
	var versionChan <-chan update.VersionStatus
	if *verbose {
		versionChan = update.GetVersion(version)
	}

	// Check dotfile status
	linker := link.New(cfg)
	results, err := linker.Check()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	allOk := true
	p.Header("dotfiles")
	for _, r := range results {
		switch r.Status {
		case link.StatusLinked:
			p.DotfileOk(r.Name, formatTargetPath(cfg, r.Target))
		case link.StatusMissing:
			p.DotfileFail(r.Name, r.Message)
			allOk = false
		case link.StatusDiff:
			p.DotfileFail(r.Name, r.Message)
			allOk = false
		}
	}

	// Wait for hooks and print results
	hookResult := <-hooksChan
	if hookResult.Err != nil {
		p.Errorf("Error running status hooks: %v", hookResult.Err)
		return 1
	}

	hooksOk := true
	if len(hookResult.Hooks) > 0 {
		p.Header("hooks")
		for _, h := range hookResult.Hooks {
			name := hooks.DisplayName(h.Name)
			if h.Ok() {
				p.HookOk(name, 0)
			} else {
				p.DotfileFail(name, "hook failed")
				hooksOk = false
			}
		}
	}

	// Show version (verbose only)
	if versionChan != nil {
		select {
		case vr := <-versionChan:
			if vr.Err == nil {
				if vr.UpToDate {
					fmt.Printf("dottie: %s (up to date)\n", version)
				} else {
					fmt.Printf("dottie: %s (update available: %s)\n", version, vr.Latest)
				}
			}
		case <-time.After(2 * time.Second):
		}
	}

	if !allOk || !hooksOk {
		return 1
	}
	return 0
}

// formatTargetPath replaces the home directory prefix with ~ for display.
func formatTargetPath(cfg *config.Config, path string) string {
	home, _ := os.UserHomeDir()
	if cfg.TargetDir == home && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
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
