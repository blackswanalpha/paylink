package datastream

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/paylink/paylink-chain/internal/events"
	"nhooyr.io/websocket"
)

// ServerConfig holds WebSocket datastream server configuration.
type ServerConfig struct {
	MaxConnections   int
	SubscriberBuffer int
	// AllowedOrigins is the list of acceptable Origin host patterns (see
	// websocket.AcceptOptions.OriginPatterns). Empty means SKIP the origin check —
	// acceptable for a devnet, not for production.
	AllowedOrigins []string
}

// DefaultServerConfig returns sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		MaxConnections:   100,
		SubscriberBuffer: 256,
	}
}

// Server manages WebSocket connections for the event datastream.
type Server struct {
	bus         *events.Bus
	config      ServerConfig
	activeConns int64
	ctx         context.Context
}

// NewServer creates a new WebSocket datastream server.
func NewServer(ctx context.Context, bus *events.Bus, config ServerConfig) *Server {
	return &Server{
		bus:    bus,
		config: config,
		ctx:    ctx,
	}
}

// Handler returns an http.HandlerFunc that upgrades connections to WebSocket.
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Reserve the slot atomically BEFORE accepting: check-then-increment would
		// let concurrent upgrades blow past MaxConnections.
		if atomic.AddInt64(&s.activeConns, 1) > int64(s.config.MaxConnections) {
			atomic.AddInt64(&s.activeConns, -1)
			http.Error(w, "too many connections", http.StatusServiceUnavailable)
			return
		}
		defer atomic.AddInt64(&s.activeConns, -1)

		opts := &websocket.AcceptOptions{}
		if len(s.config.AllowedOrigins) > 0 {
			opts.OriginPatterns = s.config.AllowedOrigins
		} else {
			opts.InsecureSkipVerify = true // devnet default: no origin check
		}

		conn, err := websocket.Accept(w, r, opts)
		if err != nil {
			log.Printf("WS accept error: %v", err)
			return
		}

		c := NewConn(s.ctx, conn, s.bus, s.config.SubscriberBuffer)
		c.Run()
	}
}

// ActiveConnections returns the current number of active WebSocket connections.
func (s *Server) ActiveConnections() int {
	return int(atomic.LoadInt64(&s.activeConns))
}
