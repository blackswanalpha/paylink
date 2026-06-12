package events

import (
	"context"
	"testing"
)

func TestLogPublisherPublish(t *testing.T) {
	p := NewLogPublisher(nil) // nil → slog.Default
	if err := p.Publish(context.Background(), "escrow.released", "PLK_1", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}
