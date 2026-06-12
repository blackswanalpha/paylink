// Package sweeper runs the periodic escrow pass: release due funded time_locks, then refund
// timed-out WAITING escrows. The actual work (and its CAS safety) lives in domain.Service.Sweep
// / the store; the sweeper is only the clock. Errors inside a pass are logged by the service —
// the loop never dies.
package sweeper

import (
	"context"
	"log/slog"
	"time"
)

// Runner is the domain surface the sweeper drives.
type Runner interface {
	Sweep(ctx context.Context)
}

// Recorder records escrow_sweeps_total (nil-safe).
type Recorder interface {
	SweepTick()
}

// Sweeper ticks Runner.Sweep on a fixed interval.
type Sweeper struct {
	run      Runner
	interval time.Duration
	m        Recorder
	log      *slog.Logger
}

// New builds a Sweeper (log may be nil → slog.Default; m may be nil → no metrics).
func New(run Runner, interval time.Duration, m Recorder, log *slog.Logger) *Sweeper {
	if log == nil {
		log = slog.Default()
	}
	return &Sweeper{run: run, interval: interval, m: m, log: log}
}

// Run ticks until ctx is cancelled, returning ctx.Err().
func (s *Sweeper) Run(ctx context.Context) error {
	s.log.Info("sweeper started", "interval", s.interval.String())
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if s.m != nil {
				s.m.SweepTick()
			}
			s.run.Sweep(ctx)
		}
	}
}
