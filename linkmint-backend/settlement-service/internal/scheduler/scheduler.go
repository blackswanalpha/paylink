// Package scheduler runs the periodic payout pass: close settlements whose T+1 cutoff has passed,
// then create+instruct a payout for each that meets the minimum. The actual work (and its CAS
// safety) lives in domain.Service.Schedule / the store; the scheduler is only the clock. Errors
// inside a pass are logged by the service — the loop never dies.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Runner is the domain surface the scheduler drives.
type Runner interface {
	Schedule(ctx context.Context)
}

// Recorder records settlement_schedule_ticks_total (nil-safe).
type Recorder interface {
	ScheduleTick()
}

// Scheduler ticks Runner.Schedule on a fixed interval.
type Scheduler struct {
	run      Runner
	interval time.Duration
	m        Recorder
	log      *slog.Logger
}

// New builds a Scheduler (log may be nil → slog.Default; m may be nil → no metrics).
func New(run Runner, interval time.Duration, m Recorder, log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{run: run, interval: interval, m: m, log: log}
}

// Run ticks until ctx is cancelled, returning ctx.Err().
func (s *Scheduler) Run(ctx context.Context) error {
	s.log.Info("scheduler started", "interval", s.interval.String())
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if s.m != nil {
				s.m.ScheduleTick()
			}
			s.run.Schedule(ctx)
		}
	}
}
