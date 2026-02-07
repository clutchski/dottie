package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clutchski/dottie/internal/config"
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
  help           Show this help message

Options:
  -n, --dry-run  Preview changes without making them
  -f, --force    Overwrite existing files without backup`)
}

func printVersion() {
	fmt.Printf("dottie %s\n", version)
	fmt.Printf("  commit: %s\n", commit)
	fmt.Printf("  built:  %s\n", date)
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	fs.BoolVar(dryRun, "dry-run", false, "dry-run")
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
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	fs.BoolVar(dryRun, "dry-run", false, "dry-run")
	force := fs.Bool("f", false, "force")
	fs.BoolVar(force, "force", false, "force")
	verbose := fs.Bool("v", false, "verbose")
	fs.BoolVar(verbose, "verbose", false, "verbose")
	_ = fs.Parse(args)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	cwd, _ := os.Getwd()
	hookRunner := hooks.New(cfg.GetHooksPath(), cwd, cfg.GetTargetDir())

	// Run pre-link hooks
	if hookErr := hookRunner.Run("pre-link", *dryRun, *verbose); hookErr != nil {
		fmt.Fprintf(os.Stderr, "Error running pre-link hooks: %v\n", hookErr)
		return 1
	}

	// Link dotfiles
	linker := link.New(cfg)
	results, err := linker.Link(*dryRun, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Print results
	for _, r := range results {
		// Always print backup notices
		if r.BackupPath != "" {
			fmt.Printf("backed up %s -> %s\n", r.Target, r.BackupPath)
		}

		// Print detailed results only with -v
		if *verbose {
			var prefix string
			switch r.Status {
			case link.StatusLinked:
				prefix = "[linked]    "
			case link.StatusWouldLink:
				prefix = "[would link]"
			case link.StatusAlreadyLinked:
				prefix = "[link]      "
			case link.StatusSkipped:
				prefix = "[skipped]   "
			case link.StatusError:
				prefix = "[error]     "
			}
			fmt.Printf("%s %s -> %s\n", prefix, filepath.Base(r.Source), r.Target)
			if r.Error != nil {
				fmt.Fprintf(os.Stderr, "            %v\n", r.Error)
			}
		}
	}

	// Run post-link hooks
	if err := hookRunner.Run("post-link", *dryRun, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error running post-link hooks: %v\n", err)
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
	fs.BoolVar(dryRun, "dry-run", false, "dry-run")
	verbose := fs.Bool("v", false, "verbose")
	fs.BoolVar(verbose, "verbose", false, "verbose")
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

	if err := hookRunner.Run(phase, *dryRun, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error running %s hooks: %v\n", phase, err)
		return 1
	}

	return 0
}

func runStatus(args []string) int {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Start hooks first (they're slower)
	cwd, _ := os.Getwd()
	hookRunner := hooks.New(cfg.GetHooksPath(), cwd, cfg.GetTargetDir())
	hooksChan := hookRunner.StartStatusCheck()

	// Print dotfiles status while hooks run
	checker := status.New(cfg)
	allOk, err := checker.Print()
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

	hooksOk := true
	if len(hookResult.Hooks) > 0 {
		fmt.Println("hooks:")
		for _, h := range hookResult.Hooks {
			if h.Ok {
				fmt.Printf("  %s[✓]%s %s\n", colorGreen, colorReset, h.Name)
			} else {
				fmt.Printf("  %s[x]%s %s\n", colorRed, colorReset, h.Name)
				hooksOk = false
			}
		}
	}

	if !allOk || !hooksOk {
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
