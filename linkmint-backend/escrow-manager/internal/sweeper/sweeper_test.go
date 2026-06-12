package sweeper

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRunner struct {
	sweeps atomic.Int64
}

func (f *fakeRunner) Sweep(context.Context) { f.sweeps.Add(1) }

type fakeRecorder struct {
	ticks atomic.Int64
}

func (f *fakeRecorder) SweepTick() { f.ticks.Add(1) }

func TestRunTicksUntilCancelled(t *testing.T) {
	run := &fakeRunner{}
	rec := &fakeRecorder{}
	s := New(run, 5*time.Millisecond, rec, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for run.sweeps.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("sweeper did not tick in time")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run should return ctx.Err(), got %v", err)
	}
	if rec.ticks.Load() < 2 {
		t.Fatalf("ticks = %d, want >= 2", rec.ticks.Load())
	}
}

func TestRunNilMetricsSafe(t *testing.T) {
	run := &fakeRunner{}
	s := New(run, time.Millisecond, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run: %v", err)
	}
	if run.sweeps.Load() == 0 {
		t.Fatal("no sweeps ran")
	}
}
