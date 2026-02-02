package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/hooks"
	dotinit "github.com/clutchski/dottie/internal/init"
	"github.com/clutchski/dottie/internal/install"
	"github.com/clutchski/dottie/internal/link"
	"github.com/clutchski/dottie/internal/status"
	"github.com/clutchski/dottie/internal/util"
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
	case "link":
		return runLink(args[1:])
	case "install":
		return runInstall(args[1:])
	case "run":
		return runRun(args[1:])
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
  link           Symlink dotfiles to home directory
  install        Install packages (brew/apt)
  run            Run install + link
  status         Show status of dotfiles
  version        Show version information
  help           Show this help message

Options:
  -n, --dry-run  Preview changes without making them
  -f, --force    Overwrite existing files without backup (link only)

Examples:
  dottie init ~/dotfiles
  dottie link -n
  dottie status`)
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

func runLink(args []string) int {
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

	// Run pre-link hooks
	hookRunner := hooks.New(cfg.GetHooksPath())
	if hookErr := hookRunner.Run(hooks.PreLink, *dryRun); hookErr != nil {
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

	// Print results only with -v
	if *verbose {
		for _, r := range results {
			var prefix string
			switch r.Status {
			case link.StatusLinked:
				prefix = "[linked]    "
			case link.StatusWouldLink:
				prefix = "[would link]"
			case link.StatusAlreadyLinked:
				prefix = "[ok]        "
			case link.StatusSkipped:
				prefix = "[skipped]   "
			case link.StatusError:
				prefix = "[error]     "
			}
			fmt.Printf("%s %s -> %s\n", prefix, filepath.Base(r.Source), r.Target)
			if r.BackupPath != "" {
				fmt.Printf("            backed up to %s\n", r.BackupPath)
			}
			if r.Error != nil {
				fmt.Fprintf(os.Stderr, "            %v\n", r.Error)
			}
		}
	}

	// Run post-link hooks
	if err := hookRunner.Run(hooks.PostLink, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error running post-link hooks: %v\n", err)
		return 1
	}

	return 0
}

func runInstall(args []string) int {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	fs.BoolVar(dryRun, "dry-run", false, "dry-run")
	_ = fs.Parse(args)

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Run pre-install hooks
	hookRunner := hooks.New(cfg.GetHooksPath())
	if err := hookRunner.Run(hooks.PreInstall, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error running pre-install hooks: %v\n", err)
		return 1
	}

	// Install packages
	installer := install.New(cfg.GetDepsPath())
	osName := util.DetectOS()

	if *dryRun {
		packages, err := installer.ListPackages(osName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("[dry-run] would install %d packages for %s\n", len(packages), osName)
		for _, p := range packages {
			fmt.Printf("  - %s\n", p)
		}
	} else {
		if err := installer.Install(osName, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	// Run post-install hooks
	if err := hookRunner.Run(hooks.PostInstall, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error running post-install hooks: %v\n", err)
		return 1
	}

	return 0
}

func runRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run")
	fs.BoolVar(dryRun, "dry-run", false, "dry-run")
	_ = fs.Parse(args)

	// Run install
	fmt.Println("==> Installing packages...")
	installArgs := []string{}
	if *dryRun {
		installArgs = append(installArgs, "-n")
	}
	if code := runInstall(installArgs); code != 0 {
		return code
	}

	fmt.Println()

	// Run link
	fmt.Println("==> Linking dotfiles...")
	linkArgs := []string{}
	if *dryRun {
		linkArgs = append(linkArgs, "-n")
	}
	if code := runLink(linkArgs); code != 0 {
		return code
	}

	return 0
}

func runStatus(args []string) int {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	checker := status.New(cfg)
	if err := checker.Print(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}

func loadConfig() (*config.Config, error) {
	// Look for .dottie.yaml in current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return config.Load(cwd)
}
