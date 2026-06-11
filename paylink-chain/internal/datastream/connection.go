package datastream

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/paylink/paylink-chain/internal/events"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const writeTimeout = 10 * time.Second

// Conn wraps a WebSocket connection with subscription state.
type Conn struct {
	ws           *websocket.Conn
	sub          *events.Subscriber
	bus          *events.Bus
	subscription *Subscription
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewConn creates a new connection wrapper.
func NewConn(ctx context.Context, ws *websocket.Conn, bus *events.Bus, bufferSize int) *Conn {
	cctx, cancel := context.WithCancel(ctx)
	return &Conn{
		ws:           ws,
		bus:          bus,
		sub:          bus.SubscribeWithBuffer(bufferSize),
		subscription: NewSubscription(nil), // default: match all
		ctx:          cctx,
		cancel:       cancel,
	}
}

// Run starts the read and write pumps. Blocks until the connection closes.
func (c *Conn) Run() {
	defer c.cleanup()

	go c.writePump()
	c.readPump()
}

func (c *Conn) readPump() {
	for {
		_, data, err := c.ws.Read(c.ctx)
		if err != nil {
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendError(msg.ID, "invalid JSON")
			continue
		}

		switch msg.Action {
		case "subscribe":
			c.handleSubscribe(msg)
		case "unsubscribe":
			c.handleUnsubscribe(msg)
		case "ping":
			c.sendPong(msg.ID)
		default:
			c.sendError(msg.ID, "unknown action: "+msg.Action)
		}
	}
}

func (c *Conn) writePump() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case evt, ok := <-c.sub.Ch():
			if !ok {
				return
			}

			c.mu.RLock()
			matches := c.subscription.Matches(evt)
			c.mu.RUnlock()

			if !matches {
				continue
			}

			evtJSON, err := json.Marshal(evt)
			if err != nil {
				log.Printf("WS event marshal error: %v", err)
				continue
			}
			msg := ServerMessage{Type: "event", Event: evtJSON}

			writeCtx, writeCancel := context.WithTimeout(c.ctx, writeTimeout)
			err = wsjson.Write(writeCtx, c.ws, msg)
			writeCancel()
			if err != nil {
				log.Printf("WS write error: %v", err)
				c.cancel()
				return
			}
		}
	}
}

func (c *Conn) handleSubscribe(msg ClientMessage) {
	c.mu.Lock()
	c.subscription = NewSubscription(msg.Filter)
	c.mu.Unlock()

	c.sendResponse(ServerMessage{Type: "subscribed", ID: msg.ID, Info: "subscription updated"})
}

func (c *Conn) handleUnsubscribe(msg ClientMessage) {
	c.mu.Lock()
	c.subscription = NewSubscription(nil)
	c.mu.Unlock()

	c.sendResponse(ServerMessage{Type: "unsubscribed", ID: msg.ID, Info: "subscription cleared"})
}

func (c *Conn) sendPong(id string) {
	c.sendResponse(ServerMessage{Type: "pong", ID: id})
}

func (c *Conn) sendError(id, errMsg string) {
	c.sendResponse(ServerMessage{Type: "error", ID: id, Error: errMsg})
}

func (c *Conn) sendResponse(msg ServerMessage) {
	writeCtx, cancel := context.WithTimeout(c.ctx, writeTimeout)
	defer cancel()
	_ = wsjson.Write(writeCtx, c.ws, msg)
}

func (c *Conn) cleanup() {
	c.bus.Unsubscribe(c.sub)
	c.ws.Close(websocket.StatusNormalClosure, "")
}
