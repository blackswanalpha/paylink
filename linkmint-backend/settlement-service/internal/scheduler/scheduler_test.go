package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRunner struct{ ticks atomic.Int32 }

func (f *fakeRunner) Schedule(context.Context) { f.ticks.Add(1) }

type fakeRec struct{ ticks atomic.Int32 }

func (r *fakeRec) ScheduleTick() { r.ticks.Add(1) }

func TestSchedulerTicksUntilCancel(t *testing.T) {
	run := &fakeRunner{}
	rec := &fakeRec{}
	s := New(run, time.Millisecond, rec, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for run.ticks.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("scheduler did not tick")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
	if rec.ticks.Load() < 2 {
		t.Fatalf("recorder ticks=%d, want >=2", rec.ticks.Load())
	}
}
