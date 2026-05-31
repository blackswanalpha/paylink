// Package wsstream implements chain.EventSource over the lVM WebSocket datastream (/ws,
// paylink-chain/internal/datastream). It subscribes to PayLink settlement events and reconnects
// with capped exponential backoff. The protocol types mirror datastream.ClientMessage /
// ServerMessage byte-for-byte.
package wsstream

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/paylink/payment-orchestrator/internal/chain"
)

const (
	readLimit      = 1 << 20 // 1 MiB per event frame
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// Keepalive: a paylink-filtered subscription can be idle for long stretches, so we proactively
// ping to detect a half-open connection (peer gone without a TCP FIN) and force a reconnect.
// These are vars (not consts) only so tests can shrink them.
var (
	pingInterval = 20 * time.Second
	pingTimeout  = 10 * time.Second
)

// clientMessage mirrors datastream.ClientMessage.
type clientMessage struct {
	Action string           `json:"action"`
	ID     string           `json:"id,omitempty"`
	Filter *subscribeFilter `json:"filter,omitempty"`
}

// subscribeFilter mirrors datastream.SubscribeFilter.
type subscribeFilter struct {
	EntityTypes []string `json:"entityTypes,omitempty"`
	EntityIDs   []string `json:"entityIds,omitempty"`
	EventKinds  []string `json:"eventKinds,omitempty"`
	Transitions []string `json:"transitions,omitempty"`
}

// serverMessage mirrors datastream.ServerMessage.
type serverMessage struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Event json.RawMessage `json:"event,omitempty"`
	Error string          `json:"error,omitempty"`
	Info  string          `json:"info,omitempty"`
}

// Source is a chain.EventSource backed by the lVM WebSocket datastream.
type Source struct {
	url string
	log *slog.Logger
}

// New builds a Source for the given ws:// URL (log may be nil → slog.Default).
func New(url string, log *slog.Logger) *Source {
	if log == nil {
		log = slog.Default()
	}
	return &Source{url: url, log: log}
}

// Run connects, subscribes, and dispatches events to handle until ctx is cancelled, reconnecting
// on transient failures with capped exponential backoff.
func (s *Source) Run(ctx context.Context, handle func(context.Context, chain.Event) error) error {
	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		connected, err := s.runOnce(ctx, handle)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if connected {
			backoff = initialBackoff // reset after a successful session
		}
		if err != nil {
			s.log.Warn("ws_session_ended", "err", err.Error(), "retry_in", backoff.String())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		// Grow backoff only for repeated dial failures; a session that connected resets above.
		if !connected {
			if backoff = backoff * 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// runOnce dials, subscribes, and reads until the connection ends. connected reports whether the
// dial succeeded (so the caller can reset backoff).
func (s *Source) runOnce(ctx context.Context, handle func(context.Context, chain.Event) error) (connected bool, err error) {
	conn, _, dialErr := websocket.Dial(ctx, s.url, nil)
	if dialErr != nil {
		return false, dialErr
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(readLimit)

	// Keepalive: ping periodically; a failed ping closes the conn, unblocking the read loop below
	// so Run reconnects. This detects half-open connections the OS hasn't yet torn down.
	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-t.C:
				pctx, cancel := context.WithTimeout(pingCtx, pingTimeout)
				err := conn.Ping(pctx)
				cancel()
				if err != nil {
					_ = conn.Close(websocket.StatusPolicyViolation, "ping timeout")
					return
				}
			}
		}
	}()

	sub := clientMessage{
		Action: "subscribe",
		ID:     "payment-orchestrator",
		Filter: &subscribeFilter{
			EntityTypes: []string{chain.EntityPayLink},
			EventKinds:  chain.SettlementEventKinds,
		},
	}
	if werr := wsjson.Write(ctx, conn, sub); werr != nil {
		return true, werr
	}
	s.log.Info("ws_subscribed", "url", s.url, "kinds", chain.SettlementEventKinds)

	for {
		var msg serverMessage
		if rerr := wsjson.Read(ctx, conn, &msg); rerr != nil {
			return true, rerr
		}
		switch msg.Type {
		case "event":
			var ev chain.Event
			if uerr := json.Unmarshal(msg.Event, &ev); uerr != nil {
				s.log.Warn("ws_event_decode_failed", "err", uerr.Error())
				continue
			}
			if herr := handle(ctx, ev); herr != nil {
				s.log.Warn("ws_event_handle_failed", "err", herr.Error())
			}
		case "error":
			s.log.Warn("ws_server_error", "error", msg.Error)
		default:
			// "subscribed", "unsubscribed", "pong" — nothing to do.
		}
	}
}
