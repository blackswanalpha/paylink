//go:build integration

package mirror_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	eventbus "github.com/paylink/eventbus-go"

	"github.com/paylink/chain-event-mirror/internal/chain"
	"github.com/paylink/chain-event-mirror/internal/metrics"
	"github.com/paylink/chain-event-mirror/internal/mirror"
)

func startRedpanda(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.2.7",
		testcontainers.WithEnv(map[string]string{"TESTCONTAINERS_RYUK_DISABLED": "true"}),
	)
	if err != nil {
		t.Fatalf("start redpanda: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	broker, err := container.KafkaSeedBroker(ctx)
	if err != nil {
		t.Fatalf("seed broker: %v", err)
	}
	return broker
}

func createTopics(t *testing.T, broker string, topics ...string) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(broker))
	if err != nil {
		t.Fatalf("admin client: %v", err)
	}
	defer cl.Close()
	resp, err := kadm.NewClient(cl).CreateTopics(context.Background(), 1, 1, nil, topics...)
	if err != nil {
		t.Fatalf("create topics: %v", err)
	}
	for _, tr := range resp.Sorted() {
		if tr.Err != nil {
			t.Fatalf("create topic %q: %v", tr.Topic, tr.Err)
		}
	}
}

// TestMirror_PublishesChainEventToBus proves a chain event handled by the mirror lands on the "chain"
// topic as a chain.<kind> envelope with the expected projected payload.
func TestMirror_PublishesChainEventToBus(t *testing.T) {
	broker := startRedpanda(t)
	createTopics(t, broker, "chain")
	ctx := context.Background()

	pub, err := eventbus.NewPublisher(
		eventbus.Config{Brokers: []string{broker}, ClientID: "mirror-test"}, "chain-event-mirror", nil,
	)
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer pub.Close()

	mr := mirror.New(pub, metrics.New(), nil)
	ev := chain.Event{
		Kind:        "paylink.verified",
		EntityType:  "paylink",
		EntityID:    "PLK_9",
		BlockHeight: 7,
		ToState:     "VERIFIED",
		Data:        json.RawMessage(`{"proofHash":"0x1"}`),
	}
	if err := mr.Handle(ctx, ev); err != nil {
		t.Fatalf("handle: %v", err)
	}

	con, err := eventbus.NewConsumer(
		eventbus.Config{Brokers: []string{broker}, ClientID: "mirror-test", GroupID: "mirror-itest"},
		[]string{"chain"}, nil,
	)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}

	type received struct {
		name    string
		payload json.RawMessage
	}
	ch := make(chan received, 1)
	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	go func() {
		_ = con.Run(runCtx, func(_ context.Context, name string, payload json.RawMessage) error {
			ch <- received{name: name, payload: payload}
			cancel()
			return nil
		})
	}()

	select {
	case got := <-ch:
		if got.name != "chain.paylink.verified" {
			t.Fatalf("name = %q", got.name)
		}
		var p map[string]any
		if err := json.Unmarshal(got.payload, &p); err != nil {
			t.Fatalf("payload decode: %v", err)
		}
		if p["entity_id"] != "PLK_9" || p["to_state"] != "VERIFIED" {
			t.Fatalf("payload = %v", p)
		}
	case <-runCtx.Done():
		t.Fatal("timed out waiting for the mirrored chain event")
	}
}
