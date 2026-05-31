package types

import (
	"encoding/json"
	"time"
)

// BlockHeader contains the metadata for a block.
type BlockHeader struct {
	Height       uint64  `json:"height"`
	Timestamp    int64   `json:"timestamp"`
	PreviousHash Hash    `json:"previousHash"`
	StateRoot    Hash    `json:"stateRoot"`
	TxRoot       Hash    `json:"txRoot"`
	ProposerAddr Address `json:"proposer"`
}

// BlockCommit contains the proposer's signature over the block header.
type BlockCommit struct {
	ProposerAddr Address `json:"proposer"`
	Signature    []byte  `json:"signature"`
}

// Block represents a complete block in the chain.
type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
	Commit       BlockCommit   `json:"commit"`
	Hash         Hash          `json:"hash"`
}

// NewBlock creates a new block with the given parameters.
func NewBlock(height uint64, prevHash Hash, txs []Transaction, stateRoot, txRoot Hash, proposer Address) *Block {
	return &Block{
		Header: BlockHeader{
			Height:       height,
			Timestamp:    time.Now().Unix(),
			PreviousHash: prevHash,
			StateRoot:    stateRoot,
			TxRoot:       txRoot,
			ProposerAddr: proposer,
		},
		Transactions: txs,
	}
}

// HeaderBytes returns the serialized header for hashing/signing.
func (b *Block) HeaderBytes() []byte {
	data, _ := json.Marshal(b.Header)
	return data
}
