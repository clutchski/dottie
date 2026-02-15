package run

import (
	"os"
	"sync"

	"github.com/clutchski/dottie/internal/config"
	"github.com/clutchski/dottie/internal/hooks"
	"github.com/clutchski/dottie/internal/link"
)

// Phase represents the current execution phase of a run.
type Phase int

const (
	PhaseIdle Phase = iota
	PhasePreHooks
	PhaseLinking
	PhasePostHooks
	PhaseDone
)

// Event is the sealed interface for typed run events.
type Event interface{ event() }

// HookEvent wraps a hook result with its phase.
type HookEvent struct {
	hooks.Result
	Phase string // "pre-link" or "post-link"
}

func (HookEvent) event() {}

// LinkEvent wraps a single link result.
type LinkEvent struct {
	link.Result
}

func (LinkEvent) event() {}

// Result is the final summary returned by Wait.
type Result struct {
	ExitCode  int
	Err       error
	PreOk     int
	PreTotal  int
	PostOk    int
	PostTotal int
	Links     link.Summary
}

// Runner executes the full dottie run sequence in a goroutine and sends
// typed events on a channel. Use Phase() and ActiveHooks() for spinner state.
type Runner struct {
	linker     *link.Linker
	hookRunner *hooks.Runner
	dryRun     bool
	force      bool

	mu     sync.RWMutex
	phase  Phase
	result Result
	done   chan struct{}
}

// New creates a new Runner.
func New(cfg *config.Config, hr *hooks.Runner, dryRun, force bool) *Runner {
	if hr == nil {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		hr = hooks.New(cfg.GetHooksPath(), hooks.EnvVars{
			"DOTTIE_ROOT": cwd,
			"DOTTIE_HOME": cfg.GetTargetDir(),
		})
	}
	return &Runner{
		linker:     link.New(cfg),
		hookRunner: hr,
		dryRun:     dryRun,
		force:      force,
		done:       make(chan struct{}),
	}
}

// Start launches the run sequence and returns a channel of events.
// The channel closes when the run is complete.
func (r *Runner) Start() <-chan Event {
	ch := make(chan Event, 64)
	go r.run(ch)
	return ch
}

// Wait blocks until the run is complete and returns the result.
func (r *Runner) Wait() Result {
	<-r.done
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.result
}

// Phase returns the current execution phase (for spinner).
func (r *Runner) Phase() Phase {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.phase
}

// ActiveHooks returns the names of currently running hooks (for spinner).
func (r *Runner) ActiveHooks() []string {
	return r.hookRunner.Active()
}

func (r *Runner) run(ch chan<- Event) {
	defer close(r.done)
	defer close(ch)

	r.setResult(r.doRun(ch))
}

func (r *Runner) doRun(ch chan<- Event) Result {
	preOk, preTotal, err := r.runHookPhase(ch, PhasePreHooks, "pre-link")
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}

	ls, err := r.runLink(ch)
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}

	pruned, err := r.runPrune(ch)
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}
	ls.Pruned = pruned

	postOk, postTotal, err := r.runHookPhase(ch, PhasePostHooks, "post-link")
	if err != nil {
		return Result{ExitCode: 1, Err: err}
	}

	exitCode := 0
	if preOk < preTotal || postOk < postTotal || ls.Errors > 0 {
		exitCode = 1
	}

	return Result{
		ExitCode:  exitCode,
		PreOk:     preOk,
		PreTotal:  preTotal,
		PostOk:    postOk,
		PostTotal: postTotal,
		Links:     ls,
	}
}

func (r *Runner) setResult(result Result) {
	r.mu.Lock()
	r.phase = PhaseDone
	r.result = result
	r.mu.Unlock()
}

func (r *Runner) runLink(ch chan<- Event) (link.Summary, error) {
	r.mu.Lock()
	r.phase = PhaseLinking
	r.mu.Unlock()

	results, err := r.linker.Link(r.dryRun, r.force)
	if err != nil {
		return link.Summary{}, err
	}

	var ls link.Summary
	for _, lr := range results {
		ch <- LinkEvent{lr}
		switch lr.Status {
		case link.StatusLinked:
			ls.Added++
		case link.StatusAlreadyLinked:
			ls.Existing++
		case link.StatusError:
			ls.Errors++
		}
	}
	return ls, nil
}

func (r *Runner) runPrune(ch chan<- Event) (int, error) {
	var dangling []link.Result
	var err error
	if r.dryRun {
		dangling, err = r.linker.FindDangling()
	} else {
		dangling, err = r.linker.Prune()
	}
	if err != nil {
		return 0, err
	}

	for _, d := range dangling {
		ch <- LinkEvent{d}
	}
	return len(dangling), nil
}

func (r *Runner) runHookPhase(ch chan<- Event, phase Phase, phaseArg string) (ok, total int, err error) {
	scripts, err := r.hookRunner.List()
	if err != nil {
		return 0, 0, err
	}
	if len(scripts) == 0 {
		return 0, 0, nil
	}

	r.mu.Lock()
	r.phase = phase
	r.mu.Unlock()

	for result := range r.hookRunner.RunScripts(scripts, phaseArg, r.dryRun) {
		ch <- HookEvent{Result: result, Phase: phaseArg}
		total++
		if result.Ok() {
			ok++
		}
	}

	return ok, total, nil
}
