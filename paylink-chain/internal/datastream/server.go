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
		if int(atomic.LoadInt64(&s.activeConns)) >= s.config.MaxConnections {
			http.Error(w, "too many connections", http.StatusServiceUnavailable)
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // Allow all origins for development
		})
		if err != nil {
			log.Printf("WS accept error: %v", err)
			return
		}

		atomic.AddInt64(&s.activeConns, 1)
		defer atomic.AddInt64(&s.activeConns, -1)

		c := NewConn(s.ctx, conn, s.bus, s.config.SubscriberBuffer)
		c.Run()
	}
}

// ActiveConnections returns the current number of active WebSocket connections.
func (s *Server) ActiveConnections() int {
	return int(atomic.LoadInt64(&s.activeConns))
}
