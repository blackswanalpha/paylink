package p2p

import "github.com/paylink/paylink-chain/internal/types"

// Host defines the P2P networking interface.
// Phase 1: no-op stub for single-validator mode.
// Phase 2: libp2p implementation with gossip.
type Host interface {
	Start() error
	Stop() error
	BroadcastBlock(block *types.Block) error
	BroadcastTx(tx *types.Transaction) error

	// Phase 2 additions
	PeerCount() int
	SyncToHead() error
	OnBlock(handler func(*types.Block))
	OnTx(handler func(*types.Transaction))
}

// NoOpHost is a stub P2P host for single-validator mode.
type NoOpHost struct{}

func NewNoOpHost() *NoOpHost     { return &NoOpHost{} }
func (h *NoOpHost) Start() error { return nil }
func (h *NoOpHost) Stop() error  { return nil }

func (h *NoOpHost) BroadcastBlock(_ *types.Block) error    { return nil }
func (h *NoOpHost) BroadcastTx(_ *types.Transaction) error { return nil }
func (h *NoOpHost) PeerCount() int                         { return 0 }
func (h *NoOpHost) SyncToHead() error                      { return nil }
func (h *NoOpHost) OnBlock(_ func(*types.Block))           {}
func (h *NoOpHost) OnTx(_ func(*types.Transaction))        {}
