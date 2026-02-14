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
	case "prune":
		return cmdPrune(args[1:])
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
  prune          Remove dangling symlinks left by deleted dotfiles
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

	p := console.New(verbosity(*verbose, *veryVerbose))
	showProgress := p.IsTTY() && p.Verbosity() == console.Quiet && !*noProgress
	progress := console.NewProgress(p.Out(), showProgress)

	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	hookRunner := newHookRunner(cfg)

	// Run pre-link hooks
	preOk, preTotal := runHooksPhaseWithProgress(hookRunner, p, progress, "pre-link", *dryRun)

	// Link dotfiles
	linker := link.New(cfg)
	results, err := linker.Link(*dryRun, *force)
	if err != nil {
		progress.Stop()
		return fatal(err)
	}

	progress.SetMessage(fmt.Sprintf("linking %d files", len(results)))

	// Prune dangling links
	dangling, err := linker.Prune()
	if err != nil {
		progress.Stop()
		return fatal(err)
	}
	pruned := 0
	if !*dryRun {
		for _, r := range dangling {
			if err := os.Remove(r.Target); err != nil && !os.IsNotExist(err) {
				continue
			}
			pruned++
		}
		if pruned > 0 {
			// Update manifest to remove pruned entries
			mp := filepath.Join(cfg.TargetDir, ".dottie.links")
			manifest, loadErr := link.LoadManifest(mp)
			if loadErr == nil {
				removed := make(map[string]bool)
				for _, r := range dangling {
					removed[r.Target] = true
				}
				var kept []string
				for _, entry := range manifest {
					if !removed[entry] {
						kept = append(kept, entry)
					}
				}
				if saveErr := link.SaveManifest(mp, kept); saveErr != nil {
					fmt.Fprintf(os.Stderr, "Error updating manifest: %v\n", saveErr)
				}
			}
		}
	}

	var ls console.LinkSummary
	p.Header("links")
	for _, r := range results {
		p.PrintLink(r)
		switch r.Status {
		case link.StatusLinked:
			ls.Added++
		case link.StatusAlreadyLinked:
			ls.Existing++
		case link.StatusError:
			ls.Errors++
		}
	}
	for _, r := range dangling {
		p.PrintLink(r)
	}
	ls.Pruned = len(dangling)

	// Run post-link hooks
	postOk, postTotal := runHooksPhaseWithProgress(hookRunner, p, progress, "post-link", *dryRun)

	progress.Stop()

	p.Summary(preOk, preTotal, ls, postOk, postTotal)

	if ls.Errors > 0 || preOk < preTotal || postOk < postTotal {
		return 1
	}
	return 0
}

func cmdPrune(args []string) int {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	dryRun := fs.Bool("n", false, "dry-run (display only, no removal)")
	if err := fs.Parse(args); err != nil {
		return fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		return fatal(err)
	}

	linker := link.New(cfg)
	results, err := linker.Prune()
	if err != nil {
		return fatal(err)
	}

	if len(results) == 0 {
		fmt.Println("No dangling symlinks found.")
		return 0
	}

	for _, r := range results {
		fmt.Printf("  %s -> %s\n", r.Target, r.Source)
	}

	if *dryRun {
		return 0
	}

	if !confirmPrune(len(results)) {
		fmt.Println("Aborted.")
		return 0
	}

	removed := make(map[string]bool)
	errCount := 0
	for _, r := range results {
		if err := os.Remove(r.Target); err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error removing %s: %v\n", r.Target, err)
				errCount++
				continue
			}
		}
		fmt.Printf("Removed %s\n", r.Target)
		removed[r.Target] = true
	}

	// Update manifest to remove pruned entries
	manifestPath := filepath.Join(cfg.TargetDir, ".dottie.links")
	manifest, err := link.LoadManifest(manifestPath)
	if err == nil {
		var kept []string
		for _, entry := range manifest {
			if !removed[entry] {
				kept = append(kept, entry)
			}
		}
		if err := link.SaveManifest(manifestPath, kept); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating manifest: %v\n", err)
		}
	}

	if errCount > 0 {
		return 1
	}
	return 0
}

func confirmPrune(count int) bool {
	s := "s"
	if count == 1 {
		s = ""
	}
	fmt.Printf("Remove %d dangling symlink%s? [y/N] ", count, s)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}
	return strings.ToLower(answer) == "y"
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

	// Check for dangling links via manifest
	dangling, err := linker.Prune()
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
	for _, r := range dangling {
		r.Target = formatTargetPath(cfg, r.Target)
		p.PrintLink(r)
		lc.Dangling++
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

	allOk := lc.Missing == 0 && lc.Diff == 0 && lc.Dangling == 0 && lc.Error == 0
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

func runHooksPhaseWithProgress(runner *hooks.Runner, p *console.Printer, prog *console.Progress, phase string, dryRun bool) (ok, total int) {
	scripts, err := runner.List()
	if err != nil {
		scripts = nil
	}
	names := hookDisplayNames(scripts)
	prog.SetTasks(phaseLabel(phase), names)

	p.Header("hooks " + phase)
	for r := range runner.RunPhase(phase, dryRun) {
		prog.FinishTask(r.Name)
		p.PrintHook(r, phase)
		total++
		if r.Ok() {
			ok++
		}
	}
	return ok, total
}

func phaseLabel(phase string) string {
	switch phase {
	case "pre-link":
		return "hooks:pre"
	case "post-link":
		return "hooks:post"
	default:
		return phase
	}
}

func hookDisplayNames(scripts []string) []string {
	names := make([]string, len(scripts))
	for i, s := range scripts {
		names[i] = hooks.DisplayName(filepath.Base(s))
	}
	return names
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
