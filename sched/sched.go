package sched

import (
	"sync"
	"time"
)

// Runner runs a scheduled tasks every specified duration.
type Runner struct {
	fn func()

	startOnce sync.Once
	stopOnce  sync.Once

	stopped chan struct{}
	ticker  *time.Ticker
	manual  chan struct{}
}

// NewRunner creates a new Runner with the given function, and runs it every
// specified duration.
//
// Note: Each runner is good for one use. After stopping, a new Runner must
// be created.
func NewRunner(fn func(), every time.Duration) *Runner {
	return &Runner{
		fn:      fn,
		stopped: make(chan struct{}),
		ticker:  time.NewTicker(every),
		manual:  make(chan struct{}, 1),
	}
}

// Start starts the runner, running its scheduled tasks on an interval.
func (r *Runner) Start() {
	r.startOnce.Do(r.run)
}

// Stop stops the runner, blocking if the task is still running.
func (r *Runner) Stop() {
	r.stopOnce.Do(r.stop)
}

// Run runs the task manually. If Run has already been called and a manual
// run is waiting to run, then the function will not run twice. In other words,
// you can only have one manual run queued at a time.
func (r *Runner) Run() {
	select {
	case r.manual <- struct{}{}:
	default:
	}
}

func (r *Runner) stop() {
	r.ticker.Stop()
	close(r.manual)
	<-r.stopped
}

func (r *Runner) run() {
	go func() {
		defer close(r.stopped)

		for {
			ok := true
			select {
			case _, ok = <-r.ticker.C:
			case _, ok = <-r.manual:
			}

			if !ok {
				return
			}

			r.runFn()
		}
	}()
}

func (r *Runner) runFn() {
	if r.fn != nil {
		r.fn()
	}
}
