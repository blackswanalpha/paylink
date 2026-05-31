package events

import (
	"context"
	"testing"
	"time"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus(DefaultBusConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Start(ctx)

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	evt := NewEvent(EventPayLinkCreated, EntityPayLink, "0xabc", 1)
	bus.Publish(evt)

	select {
	case received := <-sub.Ch():
		if received.Kind != EventPayLinkCreated {
			t.Fatalf("expected paylink.created, got %s", received.Kind)
		}
		if received.EntityID != "0xabc" {
			t.Fatalf("expected entityId 0xabc, got %s", received.EntityID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBus_FanOut(t *testing.T) {
	bus := NewBus(DefaultBusConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Start(ctx)

	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()
	sub3 := bus.Subscribe()
	defer bus.Unsubscribe(sub1)
	defer bus.Unsubscribe(sub2)
	defer bus.Unsubscribe(sub3)

	evt := NewEvent(EventTransfer, EntityAccount, "0x123", 5)
	bus.Publish(evt)

	for i, sub := range []*Subscriber{sub1, sub2, sub3} {
		select {
		case received := <-sub.Ch():
			if received.Kind != EventTransfer {
				t.Fatalf("sub %d: expected account.transfer, got %s", i, received.Kind)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub %d: timeout waiting for event", i)
		}
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus(DefaultBusConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Start(ctx)

	sub := bus.Subscribe()

	if bus.SubscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", bus.SubscriberCount())
	}

	bus.Unsubscribe(sub)

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", bus.SubscriberCount())
	}

	// Publishing after unsubscribe should not panic
	bus.Publish(NewEvent(EventBlockProduced, EntityBlock, "0xblock", 10))
}

func TestBus_SlowSubscriberDoesNotBlock(t *testing.T) {
	bus := NewBus(BusConfig{
		InternalBufferSize:   100,
		SubscriberBufferSize: 2, // tiny buffer
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Start(ctx)

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	// Publish more events than the subscriber buffer can hold
	for i := 0; i < 20; i++ {
		bus.Publish(NewEvent(EventPayLinkVoted, EntityPayLink, "0xpl", uint64(i)))
	}

	// Give time for dispatch
	time.Sleep(100 * time.Millisecond)

	// Drain what we can — should get exactly buffer-size events, rest dropped
	count := 0
	for {
		select {
		case <-sub.Ch():
			count++
		default:
			goto done
		}
	}
done:
	if count == 0 {
		t.Fatal("expected to receive some events")
	}
	if count > 2 {
		// Shouldn't exceed buffer size
		t.Fatalf("expected at most 2 events (buffer size), got %d", count)
	}
}

func TestBus_ContextCancellation(t *testing.T) {
	bus := NewBus(DefaultBusConfig())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		bus.Start(ctx)
		close(done)
	}()

	sub := bus.Subscribe()

	cancel()

	select {
	case <-done:
		// Bus stopped
	case <-time.After(time.Second):
		t.Fatal("bus did not stop after context cancellation")
	}

	// Subscriber should be closed
	select {
	case <-sub.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber not closed after bus stop")
	}
}

func TestBus_SubscriberCount(t *testing.T) {
	bus := NewBus(DefaultBusConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Start(ctx)

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0, got %d", bus.SubscriberCount())
	}

	s1 := bus.Subscribe()
	s2 := bus.Subscribe()

	if bus.SubscriberCount() != 2 {
		t.Fatalf("expected 2, got %d", bus.SubscriberCount())
	}

	bus.Unsubscribe(s1)
	if bus.SubscriberCount() != 1 {
		t.Fatalf("expected 1, got %d", bus.SubscriberCount())
	}

	bus.Unsubscribe(s2)
	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0, got %d", bus.SubscriberCount())
	}
}

func TestEvent_WithTransition(t *testing.T) {
	evt := NewEvent(EventPayLinkCreated, EntityPayLink, "0xabc", 1).
		WithTransition("NONE", "CREATED", "Create").
		WithTx("0xtxhash").
		WithData(PayLinkCreatedData{
			Creator:  "0xcreator",
			Receiver: "0xreceiver",
			Amount:   1000,
		})

	if evt.FromState != "NONE" {
		t.Fatalf("expected NONE, got %s", evt.FromState)
	}
	if evt.ToState != "CREATED" {
		t.Fatalf("expected CREATED, got %s", evt.ToState)
	}
	if evt.Transition != "Create" {
		t.Fatalf("expected Create, got %s", evt.Transition)
	}
	if evt.TxHash != "0xtxhash" {
		t.Fatalf("expected 0xtxhash, got %s", evt.TxHash)
	}
	if evt.Data == nil {
		t.Fatal("expected data to be set")
	}
	if evt.Sequence == 0 {
		t.Fatal("expected non-zero sequence")
	}
}

func TestEvent_MonotonicSequence(t *testing.T) {
	e1 := NewEvent(EventTransfer, EntityAccount, "a", 0)
	e2 := NewEvent(EventTransfer, EntityAccount, "b", 0)
	e3 := NewEvent(EventTransfer, EntityAccount, "c", 0)

	if e2.Sequence <= e1.Sequence {
		t.Fatal("sequence should be monotonically increasing")
	}
	if e3.Sequence <= e2.Sequence {
		t.Fatal("sequence should be monotonically increasing")
	}
}
