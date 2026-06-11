package types

import (
	"encoding/json"
	"fmt"
)

// MaxTxPayloadBytes caps a transaction payload at every admission point (RPC, P2P).
// The largest legitimate payload (CreatePayLink with a full rule set) is well under
// 4KB; 32KB leaves headroom without letting clients bloat blocks.
const MaxTxPayloadBytes = 32 * 1024

// TxType identifies the type of transaction.
type TxType uint8

const (
	TxCreatePayLink     TxType = 1
	TxSubmitValidation  TxType = 2
	TxCancelPayLink     TxType = 3
	TxFailPayLink       TxType = 4 // admin only
	TxTransfer          TxType = 5
	TxStake             TxType = 6
	TxInitiateUnstake   TxType = 7
	TxCompleteUnstake   TxType = 8
	TxSlash             TxType = 9  // admin only
	TxDistributeReward  TxType = 10 // admin only
	TxRegisterVRFKey    TxType = 11 // validator registers VRF public key
	TxSubmitEvidence    TxType = 12 // anyone submits slashing evidence
	TxTransferPayLink   TxType = 13 // owner transfers paylink to new owner
	TxApprovePayLink    TxType = 14 // owner approves address for single paylink
	TxSetApprovalForAll TxType = 15 // owner approves operator for all their paylinks
)

func (t TxType) String() string {
	switch t {
	case TxCreatePayLink:
		return "CreatePayLink"
	case TxSubmitValidation:
		return "SubmitValidation"
	case TxCancelPayLink:
		return "CancelPayLink"
	case TxFailPayLink:
		return "FailPayLink"
	case TxTransfer:
		return "Transfer"
	case TxStake:
		return "Stake"
	case TxInitiateUnstake:
		return "InitiateUnstake"
	case TxCompleteUnstake:
		return "CompleteUnstake"
	case TxSlash:
		return "Slash"
	case TxDistributeReward:
		return "DistributeReward"
	case TxRegisterVRFKey:
		return "RegisterVRFKey"
	case TxSubmitEvidence:
		return "SubmitEvidence"
	case TxTransferPayLink:
		return "TransferPayLink"
	case TxApprovePayLink:
		return "ApprovePayLink"
	case TxSetApprovalForAll:
		return "SetApprovalForAll"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

// Transaction represents a signed transaction on the PayLink chain.
//
// PubKey carries the sender's uncompressed P-256 public key (65 bytes: 0x04 || X || Y).
// P-256 ECDSA has no public-key recovery, so the key must travel with the transaction;
// it is bound to From by the address derivation check (PubkeyToAddress(PubKey) == From),
// which is why PubKey does not need to be covered by SignableBytes.
type Transaction struct {
	Type      TxType          `json:"type"`
	From      Address         `json:"from"`
	Nonce     uint64          `json:"nonce"`
	Payload   json.RawMessage `json:"payload"`
	PubKey    []byte          `json:"pubKey,omitempty"`
	Signature []byte          `json:"signature"`
	Hash      Hash            `json:"hash"`
}

// ── Payload structs ──

// CreatePayLinkPayload corresponds to TxCreatePayLink.
type CreatePayLinkPayload struct {
	PayLinkID    Hash            `json:"paylinkId"`
	Receiver     Address         `json:"receiver"`
	Amount       uint64          `json:"amount"`
	Expiry       int64           `json:"expiry"` // Unix timestamp
	MetadataHash Hash            `json:"metadataHash"`
	Rules        json.RawMessage `json:"rules,omitempty"` // optional rule set
}

// SubmitValidationPayload corresponds to TxSubmitValidation.
type SubmitValidationPayload struct {
	PayLinkID Hash `json:"paylinkId"`
	ProofHash Hash `json:"proofHash"`
}

// CancelPayLinkPayload corresponds to TxCancelPayLink.
type CancelPayLinkPayload struct {
	PayLinkID Hash `json:"paylinkId"`
}

// FailPayLinkPayload corresponds to TxFailPayLink (admin only).
type FailPayLinkPayload struct {
	PayLinkID Hash `json:"paylinkId"`
}

// TransferPayload corresponds to TxTransfer.
type TransferPayload struct {
	To     Address `json:"to"`
	Amount uint64  `json:"amount"`
}

// StakePayload corresponds to TxStake.
type StakePayload struct {
	Amount uint64 `json:"amount"`
}

// InitiateUnstakePayload corresponds to TxInitiateUnstake.
type InitiateUnstakePayload struct {
	Amount uint64 `json:"amount"`
}

// CompleteUnstakePayload corresponds to TxCompleteUnstake (no payload fields).
type CompleteUnstakePayload struct{}

// SlashPayload corresponds to TxSlash (admin only).
type SlashPayload struct {
	Validator Address `json:"validator"`
	Amount    uint64  `json:"amount"`
	Reason    string  `json:"reason"`
}

// DistributeRewardPayload corresponds to TxDistributeReward (admin only).
type DistributeRewardPayload struct {
	Validator Address `json:"validator"`
	Amount    uint64  `json:"amount"`
}

// RegisterVRFKeyPayload corresponds to TxRegisterVRFKey.
type RegisterVRFKeyPayload struct {
	VRFPublicKey []byte `json:"vrfPublicKey"` // 32-byte ED25519 public key
}

// SubmitEvidencePayload corresponds to TxSubmitEvidence.
type SubmitEvidencePayload struct {
	EvidenceType string          `json:"evidenceType"` // "double_sign", "equivocation", "liveness"
	Validator    Address         `json:"validator"`
	Data         json.RawMessage `json:"data"` // evidence-type-specific data
}

// TransferPayLinkPayload corresponds to TxTransferPayLink.
type TransferPayLinkPayload struct {
	PayLinkID Hash    `json:"paylinkId"`
	To        Address `json:"to"`
}

// ApprovePayLinkPayload corresponds to TxApprovePayLink.
type ApprovePayLinkPayload struct {
	PayLinkID Hash    `json:"paylinkId"`
	Approved  Address `json:"approved"` // zero address = revoke
}

// SetApprovalForAllPayload corresponds to TxSetApprovalForAll.
type SetApprovalForAllPayload struct {
	Operator Address `json:"operator"`
	Approved bool    `json:"approved"` // true = grant, false = revoke
}

// DecodePayload unmarshals the transaction's raw payload into the appropriate struct.
func (tx *Transaction) DecodePayload() (interface{}, error) {
	switch tx.Type {
	case TxCreatePayLink:
		var p CreatePayLinkPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxSubmitValidation:
		var p SubmitValidationPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxCancelPayLink:
		var p CancelPayLinkPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxFailPayLink:
		var p FailPayLinkPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxTransfer:
		var p TransferPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxStake:
		var p StakePayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxInitiateUnstake:
		var p InitiateUnstakePayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxCompleteUnstake:
		var p CompleteUnstakePayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxSlash:
		var p SlashPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxDistributeReward:
		var p DistributeRewardPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxRegisterVRFKey:
		var p RegisterVRFKeyPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxSubmitEvidence:
		var p SubmitEvidencePayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxTransferPayLink:
		var p TransferPayLinkPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxApprovePayLink:
		var p ApprovePayLinkPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	case TxSetApprovalForAll:
		var p SetApprovalForAllPayload
		return &p, json.Unmarshal(tx.Payload, &p)
	default:
		return nil, fmt.Errorf("unknown tx type: %d", tx.Type)
	}
}

// SignableBytes returns the bytes that should be signed (everything except signature and hash).
func (tx *Transaction) SignableBytes() []byte {
	data, _ := json.Marshal(struct {
		Type    TxType          `json:"type"`
		From    Address         `json:"from"`
		Nonce   uint64          `json:"nonce"`
		Payload json.RawMessage `json:"payload"`
	}{
		Type:    tx.Type,
		From:    tx.From,
		Nonce:   tx.Nonce,
		Payload: tx.Payload,
	})
	return data
}
