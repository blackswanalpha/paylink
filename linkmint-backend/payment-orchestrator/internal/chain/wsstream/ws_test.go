package wsstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/paylink/payment-orchestrator/internal/chain"
)

func TestSourceReceivesEvent(t *testing.T) {
	// Shrink the keepalive so the ping path runs during the test (server auto-replies pong).
	origInterval, origTimeout := pingInterval, pingTimeout
	pingInterval, pingTimeout = 30*time.Millisecond, 500*time.Millisecond
	defer func() { pingInterval, pingTimeout = origInterval, origTimeout }()

	received := make(chan chain.Event, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()

		var sub map[string]any
		if err := wsjson.Read(ctx, c, &sub); err != nil {
			return
		}
		_ = wsjson.Write(ctx, c, map[string]any{"type": "subscribed", "info": "ok"})

		ev, _ := json.Marshal(map[string]any{
			"seq": 1, "kind": "paylink.verified", "entityType": "paylink",
			"entityId": "0xabc", "toState": "VERIFIED",
		})
		_ = wsjson.Write(ctx, c, map[string]any{"type": "event", "event": json.RawMessage(ev)})
		// exercise the error and default branches too
		_ = wsjson.Write(ctx, c, map[string]any{"type": "error", "error": "harmless"})
		_ = wsjson.Write(ctx, c, map[string]any{"type": "pong"})
		<-ctx.Done()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = New(wsURL, nil).Run(ctx, func(_ context.Context, ev chain.Event) error {
			received <- ev
			return nil
		})
	}()

	select {
	case ev := <-received:
		if ev.EntityID != "0xabc" || ev.ToState != "VERIFIED" {
			t.Fatalf("unexpected event %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSourceStopsOnContextCancel(t *testing.T) {
	// Unroutable endpoint: dial fails, then the backoff select returns on ctx timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	err := New("ws://127.0.0.1:1/ws", nil).Run(ctx, func(context.Context, chain.Event) error { return nil })
	if err == nil {
		t.Fatal("expected a context error after cancellation")
	}
}
