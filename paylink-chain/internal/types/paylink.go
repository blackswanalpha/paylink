package types

import "encoding/json"

// Status represents the lifecycle state of a PayLink.
type Status uint8

const (
	StatusNone      Status = 0
	StatusCreated   Status = 1
	StatusVerified  Status = 2
	StatusFailed    Status = 3
	StatusCancelled Status = 4
)

func (s Status) String() string {
	switch s {
	case StatusNone:
		return "NONE"
	case StatusCreated:
		return "CREATED"
	case StatusVerified:
		return "VERIFIED"
	case StatusFailed:
		return "FAILED"
	case StatusCancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

// PayLink represents a payment link record on-chain.
type PayLink struct {
	ID            Hash            `json:"id"`
	Creator       Address         `json:"creator"`
	Receiver      Address         `json:"receiver"`
	Owner         Address         `json:"owner"`                   // current owner (initially = Creator)
	Approved      Address         `json:"approved"`                // single-paylink approval (zero = none)
	Amount        uint64          `json:"amount"`
	Expiry        int64           `json:"expiry"`                  // Unix timestamp
	Status        Status          `json:"status"`
	MetadataHash  Hash            `json:"metadataHash"`
	CreatedAt     int64           `json:"createdAt"`
	VoteCount     uint64          `json:"voteCount"`
	TransferCount uint64          `json:"transferCount"`           // number of ownership transfers
	Rules         json.RawMessage `json:"rules,omitempty"`         // immutable after creation
}
