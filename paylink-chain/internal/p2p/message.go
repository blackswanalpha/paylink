package p2p

import (
	"encoding/json"
	"fmt"

	"github.com/paylink/paylink-chain/internal/types"
)

// Message types for the wire protocol.
const (
	MsgTypeBlock    = "block"
	MsgTypeTx       = "tx"
	MsgTypeSyncReq  = "sync_request"
	MsgTypeSyncResp = "sync_response"
)

// Envelope wraps a message with its type for serialization.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// BlockMessage wraps a block for gossip.
type BlockMessage struct {
	Block *types.Block `json:"block"`
}

// TxMessage wraps a transaction for gossip.
type TxMessage struct {
	Transaction *types.Transaction `json:"transaction"`
}

// SyncRequest asks a peer for a range of blocks.
type SyncRequest struct {
	FromHeight uint64 `json:"fromHeight"`
	ToHeight   uint64 `json:"toHeight"`
}

// SyncResponse returns a batch of blocks.
type SyncResponse struct {
	Blocks []*types.Block `json:"blocks"`
}

// EncodeEnvelope serializes a message into an envelope.
func EncodeEnvelope(msgType string, data interface{}) ([]byte, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	return json.Marshal(Envelope{Type: msgType, Data: raw})
}

// DecodeEnvelope deserializes an envelope.
func DecodeEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}
