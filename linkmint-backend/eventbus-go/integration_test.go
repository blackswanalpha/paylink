//go:build integration

package eventbus_test

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
)

// startRedpanda boots a single-node Redpanda and returns its Kafka seed broker. Ryuk is disabled
// because the container is explicitly terminated in t.Cleanup (mirrors the other services' style).
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

// createTopics pre-creates topics the way the deployed stack's redpanda-init step does (rpk topic
// create), so produce never races topic auto-creation.
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

func TestPublishConsume_RoundTrip(t *testing.T) {
	broker := startRedpanda(t)
	ctx := context.Background()
	createTopics(t, broker, "paylink")

	pub, err := eventbus.NewPublisher(eventbus.Config{Brokers: []string{broker}, ClientID: "test"}, "test-suite", nil)
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer pub.Close()

	if err := pub.Publish(ctx, "paylink.verified", "PLK_1", map[string]any{"pl_id": "PLK_1", "amount": "500"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	con, err := eventbus.NewConsumer(
		eventbus.Config{Brokers: []string{broker}, ClientID: "test", GroupID: "rt-group"},
		[]string{"paylink"}, nil,
	)
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}

	type got struct {
		name    string
		payload string
	}
	ch := make(chan got, 1)
	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	go func() {
		_ = con.Run(runCtx, func(_ context.Context, name string, payload json.RawMessage) error {
			ch <- got{name: name, payload: string(payload)}
			cancel()
			return nil
		})
	}()

	select {
	case g := <-ch:
		if g.name != "paylink.verified" {
			t.Fatalf("name = %q", g.name)
		}
		if g.payload != `{"amount":"500","pl_id":"PLK_1"}` {
			t.Fatalf("payload = %s", g.payload)
		}
	case <-runCtx.Done():
		t.Fatal("timed out waiting for event")
	}
}

// TestConsume_RedeliversOnHandleError proves at-least-once: a handler that errors does not commit,
// so a fresh consumer in the same group receives the event again.
func TestConsume_RedeliversOnHandleError(t *testing.T) {
	broker := startRedpanda(t)
	ctx := context.Background()
	createTopics(t, broker, "payment")

	pub, err := eventbus.NewPublisher(eventbus.Config{Brokers: []string{broker}, ClientID: "test"}, "test-suite", nil)
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer pub.Close()
	if err := pub.Publish(ctx, "payment.failed", "PMT_1", map[string]any{"reason": "boom"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	cfg := eventbus.Config{Brokers: []string{broker}, ClientID: "test", GroupID: "redeliver-group"}

	// First consumer: handler errors → no commit.
	con1, err := eventbus.NewConsumer(cfg, []string{"payment"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen1 := make(chan struct{}, 1)
	c1ctx, c1cancel := context.WithTimeout(ctx, 30*time.Second)
	go func() {
		_ = con1.Run(c1ctx, func(_ context.Context, _ string, _ json.RawMessage) error {
			select {
			case seen1 <- struct{}{}:
			default:
			}
			c1cancel()
			return errContext
		})
	}()
	select {
	case <-seen1:
	case <-c1ctx.Done():
		t.Fatal("first consumer never saw the event")
	}
	<-c1ctx.Done() // ensure con1 has stopped (no commit happened)

	// Second consumer in the same group: must receive the uncommitted event again.
	con2, err := eventbus.NewConsumer(cfg, []string{"payment"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(chan struct{}, 1)
	c2ctx, c2cancel := context.WithTimeout(ctx, 45*time.Second)
	defer c2cancel()
	go func() {
		_ = con2.Run(c2ctx, func(_ context.Context, _ string, _ json.RawMessage) error {
			seen2 <- struct{}{}
			c2cancel()
			return nil
		})
	}()
	select {
	case <-seen2:
	case <-c2ctx.Done():
		t.Fatal("event was not redelivered to the second consumer")
	}
}

var errContext = context.DeadlineExceeded // a sentinel non-nil error for the failing handler
