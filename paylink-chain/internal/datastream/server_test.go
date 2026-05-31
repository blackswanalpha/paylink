package datastream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/internal/events"
	"github.com/paylink/paylink-chain/internal/fsm"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func setupTestServer(t *testing.T) (context.Context, context.CancelFunc, *events.Bus, *httptest.Server) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	bus := events.NewBus(events.BusConfig{
		InternalBufferSize:   256,
		SubscriberBufferSize: 64,
	})
	go bus.Start(ctx)

	srv := NewServer(ctx, bus, ServerConfig{
		MaxConnections:   5,
		SubscriberBuffer: 64,
	})

	ts := httptest.NewServer(srv.Handler())

	return ctx, cancel, bus, ts
}

func dialWS(t *testing.T, ctx context.Context, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + ts.URL[4:] // http -> ws
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func readServerMsg(t *testing.T, ctx context.Context, conn *websocket.Conn) ServerMessage {
	t.Helper()
	rCtx, rCancel := context.WithTimeout(ctx, 2*time.Second)
	defer rCancel()
	var msg ServerMessage
	if err := wsjson.Read(rCtx, conn, &msg); err != nil {
		t.Fatalf("read: %v", err)
	}
	return msg
}

func writeClientMsg(t *testing.T, ctx context.Context, conn *websocket.Conn, msg ClientMessage) {
	t.Helper()
	wCtx, wCancel := context.WithTimeout(ctx, 2*time.Second)
	defer wCancel()
	if err := wsjson.Write(wCtx, conn, msg); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ── Tests ──

func TestWS_ConnectAndReceiveEvent(t *testing.T) {
	ctx, cancel, bus, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait for subscription to register
	time.Sleep(50 * time.Millisecond)

	// Publish an event
	evt := events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, "0xtest123", 1).
		WithTransition(fsm.PayLinkNone, fsm.PayLinkCreated, fsm.PayLinkCreate)
	bus.Publish(evt)

	// Read the event
	msg := readServerMsg(t, ctx, conn)
	if msg.Type != "event" {
		t.Fatalf("expected event type, got %s", msg.Type)
	}

	var receivedEvt events.Event
	if err := json.Unmarshal(msg.Event, &receivedEvt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if receivedEvt.Kind != events.EventPayLinkCreated {
		t.Fatalf("expected paylink.created, got %s", receivedEvt.Kind)
	}
	if receivedEvt.EntityID != "0xtest123" {
		t.Fatalf("expected entityId 0xtest123, got %s", receivedEvt.EntityID)
	}
	if receivedEvt.FromState != fsm.PayLinkNone {
		t.Fatalf("expected fromState NONE, got %s", receivedEvt.FromState)
	}
	if receivedEvt.ToState != fsm.PayLinkCreated {
		t.Fatalf("expected toState CREATED, got %s", receivedEvt.ToState)
	}
}

func TestWS_SubscribeWithFilter(t *testing.T) {
	ctx, cancel, bus, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to only paylink events
	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "subscribe",
		ID:     "req-1",
		Filter: &SubscribeFilter{
			EntityTypes: []string{"paylink"},
		},
	})

	// Read subscription acknowledgment
	ack := readServerMsg(t, ctx, conn)
	if ack.Type != "subscribed" {
		t.Fatalf("expected subscribed, got %s", ack.Type)
	}
	if ack.ID != "req-1" {
		t.Fatalf("expected id req-1, got %s", ack.ID)
	}

	// Publish a validator event (should be filtered out)
	bus.Publish(events.NewEvent(events.EventValidatorStaked, events.EntityValidator, "0xval", 1))

	// Publish a paylink event (should be received)
	bus.Publish(events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, "0xpl", 1))

	// Should receive the paylink event
	msg := readServerMsg(t, ctx, conn)
	if msg.Type != "event" {
		t.Fatalf("expected event, got %s", msg.Type)
	}

	var evt events.Event
	json.Unmarshal(msg.Event, &evt)
	if evt.Kind != events.EventPayLinkCreated {
		t.Fatalf("expected paylink.created, got %s", evt.Kind)
	}
}

func TestWS_SubscribeByEntityID(t *testing.T) {
	ctx, cancel, bus, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to specific entity ID
	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "subscribe",
		ID:     "req-filter",
		Filter: &SubscribeFilter{
			EntityIDs: []string{"0xtarget"},
		},
	})
	readServerMsg(t, ctx, conn) // ack

	// Publish event for wrong ID (filtered out)
	bus.Publish(events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, "0xother", 1))

	// Publish event for matching ID
	bus.Publish(events.NewEvent(events.EventPayLinkVerified, events.EntityPayLink, "0xtarget", 2))

	msg := readServerMsg(t, ctx, conn)
	var evt events.Event
	json.Unmarshal(msg.Event, &evt)
	if evt.EntityID != "0xtarget" {
		t.Fatalf("expected 0xtarget, got %s", evt.EntityID)
	}
	if evt.Kind != events.EventPayLinkVerified {
		t.Fatalf("expected paylink.verified, got %s", evt.Kind)
	}
}

func TestWS_Unsubscribe(t *testing.T) {
	ctx, cancel, bus, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to only paylink events
	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "subscribe",
		ID:     "sub",
		Filter: &SubscribeFilter{
			EntityTypes: []string{"paylink"},
		},
	})
	readServerMsg(t, ctx, conn) // ack

	// Unsubscribe (resets to match all)
	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "unsubscribe",
		ID:     "unsub",
	})

	ack := readServerMsg(t, ctx, conn)
	if ack.Type != "unsubscribed" {
		t.Fatalf("expected unsubscribed, got %s", ack.Type)
	}
	if ack.ID != "unsub" {
		t.Fatalf("expected id unsub, got %s", ack.ID)
	}

	// Now should receive validator events too (reset to match-all)
	bus.Publish(events.NewEvent(events.EventValidatorStaked, events.EntityValidator, "0xval", 3))

	msg := readServerMsg(t, ctx, conn)
	var evt events.Event
	json.Unmarshal(msg.Event, &evt)
	if evt.Kind != events.EventValidatorStaked {
		t.Fatalf("expected validator.staked, got %s", evt.Kind)
	}
}

func TestWS_PingPong(t *testing.T) {
	ctx, cancel, _, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "ping",
		ID:     "ping-1",
	})

	msg := readServerMsg(t, ctx, conn)
	if msg.Type != "pong" {
		t.Fatalf("expected pong, got %s", msg.Type)
	}
	if msg.ID != "ping-1" {
		t.Fatalf("expected id ping-1, got %s", msg.ID)
	}
}

func TestWS_UnknownAction(t *testing.T) {
	ctx, cancel, _, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "foobar",
		ID:     "bad-1",
	})

	msg := readServerMsg(t, ctx, conn)
	if msg.Type != "error" {
		t.Fatalf("expected error, got %s", msg.Type)
	}
	if msg.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestWS_MaxConnections(t *testing.T) {
	ctx, cancel, _, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	// Open 5 connections (the max)
	conns := make([]*websocket.Conn, 5)
	for i := 0; i < 5; i++ {
		conns[i] = dialWS(t, ctx, ts)
		defer conns[i].Close(websocket.StatusNormalClosure, "")
	}

	// Give time for connections to register
	time.Sleep(100 * time.Millisecond)

	// 6th connection should be rejected
	wsURL := "ws" + ts.URL[4:]
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected 6th connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 status, got %d", resp.StatusCode)
	}
}

func TestWS_EventKindFilter(t *testing.T) {
	ctx, cancel, bus, ts := setupTestServer(t)
	defer cancel()
	defer ts.Close()

	conn := dialWS(t, ctx, ts)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to only paylink.verified events
	writeClientMsg(t, ctx, conn, ClientMessage{
		Action: "subscribe",
		ID:     "ek",
		Filter: &SubscribeFilter{
			EventKinds: []string{"paylink.verified"},
		},
	})
	readServerMsg(t, ctx, conn) // ack

	// Publish created (should be filtered)
	bus.Publish(events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, "0xpl", 1))

	// Publish verified (should pass)
	bus.Publish(events.NewEvent(events.EventPayLinkVerified, events.EntityPayLink, "0xpl", 2))

	msg := readServerMsg(t, ctx, conn)
	var evt events.Event
	json.Unmarshal(msg.Event, &evt)
	if evt.Kind != events.EventPayLinkVerified {
		t.Fatalf("expected paylink.verified, got %s", evt.Kind)
	}
}

// ── Subscription Unit Tests ──

func TestSubscription_MatchAll(t *testing.T) {
	sub := NewSubscription(nil)
	evt := events.NewEvent(events.EventTransfer, events.EntityAccount, "0x", 0)
	if !sub.Matches(evt) {
		t.Fatal("nil filter should match all")
	}
}

func TestSubscription_EmptyFilter(t *testing.T) {
	sub := NewSubscription(&SubscribeFilter{})
	evt := events.NewEvent(events.EventTransfer, events.EntityAccount, "0x", 0)
	if !sub.Matches(evt) {
		t.Fatal("empty filter should match all")
	}
}

func TestSubscription_EntityTypeFilter(t *testing.T) {
	sub := NewSubscription(&SubscribeFilter{
		EntityTypes: []string{"paylink"},
	})

	plEvt := events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, "0x", 0)
	valEvt := events.NewEvent(events.EventValidatorStaked, events.EntityValidator, "0x", 0)

	if !sub.Matches(plEvt) {
		t.Fatal("should match paylink event")
	}
	if sub.Matches(valEvt) {
		t.Fatal("should not match validator event")
	}
}

func TestSubscription_MultiDimensionAND(t *testing.T) {
	sub := NewSubscription(&SubscribeFilter{
		EntityTypes: []string{"paylink"},
		EventKinds:  []string{"paylink.verified"},
	})

	// Paylink + verified: match
	evt1 := events.NewEvent(events.EventPayLinkVerified, events.EntityPayLink, "0x", 0)
	if !sub.Matches(evt1) {
		t.Fatal("should match paylink+verified")
	}

	// Paylink + created: no match (wrong kind)
	evt2 := events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, "0x", 0)
	if sub.Matches(evt2) {
		t.Fatal("should not match paylink+created")
	}

	// Validator + any: no match (wrong type)
	evt3 := events.NewEvent(events.EventValidatorStaked, events.EntityValidator, "0x", 0)
	if sub.Matches(evt3) {
		t.Fatal("should not match validator")
	}
}
