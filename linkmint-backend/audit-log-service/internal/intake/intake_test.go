package intake

import (
	"context"
	"testing"
	"time"

	"github.com/paylink/audit-log-service/internal/domain"
)

func TestNoopSourceBlocksUntilCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NoopSource{}.Run(ctx, func(context.Context, domain.AppendInput) error { return nil })
	}()

	select {
	case <-done:
		t.Fatal("Run returned before context was cancelled")
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a context error after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
