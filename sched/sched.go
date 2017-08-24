package sched

import (
	"sync"
	"time"
)

type Runner struct {
	fn func()

	startOnce sync.Once
	stopOnce  sync.Once

	stopped chan struct{}
	ticker  *time.Ticker
	manual  chan struct{}
}

func NewRunner(fn func(), every time.Duration) *Runner {
	return &Runner{
		fn:      fn,
		stopped: make(chan struct{}),
		ticker:  time.NewTicker(every),
		manual:  make(chan struct{}, 1),
	}
}

func (r *Runner) Start() {
	r.startOnce.Do(r.run)
}

func (r *Runner) Stop() {
	r.stopOnce.Do(r.stop)
}

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
		defer func() {
			r.stopped <- struct{}{}
		}()

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
