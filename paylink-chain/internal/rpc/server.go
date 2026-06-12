package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Server is the JSON-RPC HTTP server.
type Server struct {
	handlers   *Handlers
	server     *http.Server
	corsOrigin string // Access-Control-Allow-Origin value; empty disables the header
}

// NewServer creates a new JSON-RPC server.
// An optional wsHandler is mounted at /ws if non-nil.
func NewServer(handlers *Handlers, addr string, wsHandler ...http.HandlerFunc) *Server {
	s := &Server{
		handlers:   handlers,
		corsOrigin: "*", // devnet default; production sets a real origin (or "") via SetCORSOrigin
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRPC)

	if len(wsHandler) > 0 && wsHandler[0] != nil {
		mux.HandleFunc("/ws", wsHandler[0])
		log.Println("WebSocket datastream enabled at /ws")
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

// SetCORSOrigin sets the Access-Control-Allow-Origin header value.
// Empty string disables the header entirely.
func (s *Server) SetCORSOrigin(origin string) {
	s.corsOrigin = origin
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Printf("JSON-RPC server listening on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("rpc server: %w", err)
	}
	return nil
}

// Stop gracefully stops the server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// CORS headers
	if s.corsOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	}
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		writeResponse(w, NewErrorResponse(nil, ErrCodeParse, "failed to read request"))
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeResponse(w, NewErrorResponse(nil, ErrCodeParse, "invalid JSON"))
		return
	}

	if req.JSONRPC != "2.0" {
		writeResponse(w, NewErrorResponse(req.ID, ErrCodeInvalidRequest, "invalid jsonrpc version"))
		return
	}

	resp := s.handlers.Dispatch(&req)
	writeResponse(w, resp)
}

func writeResponse(w http.ResponseWriter, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
