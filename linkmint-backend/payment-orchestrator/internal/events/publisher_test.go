package events

import (
	"context"
	"testing"
)

func TestLogPublisherPublish(t *testing.T) {
	p := NewLogPublisher(nil) // nil → slog.Default
	if err := p.Publish(context.Background(), "payment.settled", "0xabc", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}
