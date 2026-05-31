package events_test

import (
	"context"
	"testing"

	"github.com/paylink/proof-validator/internal/events"
)

func TestLogPublisher_Publish(t *testing.T) {
	// nil logger → defaults to slog.Default; Publish is fire-and-log and never fails.
	p := events.NewLogPublisher(nil)
	if err := p.Publish(context.Background(), events.ProofReceived, "0xkey", map[string]any{"rail": "mpesa"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}
