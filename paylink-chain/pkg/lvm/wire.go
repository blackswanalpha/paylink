// Package lvm is the public, byte-exact client surface for the lVM wire format and crypto.
//
// It exists so off-chain Go services (the proof-validator in work03, adapters in work04, and
// later settlement/wallet/reconciliation) can construct, sign, and broadcast transactions that
// are byte-identical to what the chain produces — WITHOUT re-deriving the format. Go's internal/
// rule forbids importing paylink-chain/internal/* from another module; this package lives in the
// same module as internal/{types,crypto}, so it can re-export them through a stable, public API.
// External services depend on this package via a go.mod replace directive.
//
// Everything here is a thin alias or wrapper over internal/{types,crypto}; there is no second
// implementation of the wire format. The canonical formulas that the chain does NOT itself define
// (ProofHash, the unsigned-tx assembly) are centralized here so every component agrees.
package lvm

import (
	"encoding/json"
	"fmt"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/types"
)

// ── Wire types (aliases — identical wire identity, not new types) ──

// Transaction is the lVM transaction envelope. Its SignableBytes()/wire JSON are authoritative.
type Transaction = types.Transaction

// TxType identifies a transaction's kind.
type TxType = types.TxType

// SubmitValidationPayload is the TxSubmitValidation (settlement) payload.
type SubmitValidationPayload = types.SubmitValidationPayload

// CreatePayLinkPayload is the TxCreatePayLink payload (used by tests/clients that create PayLinks).
type CreatePayLinkPayload = types.CreatePayLinkPayload

// StakePayload is the TxStake payload (used by the devnet auto-stake path).
type StakePayload = types.StakePayload

// Address is a 20-byte account address (marshals as a 0x-hex JSON string).
type Address = types.Address

// Hash is a 32-byte hash (marshals as a 0x-hex JSON string).
type Hash = types.Hash

// ── Transaction-type constants the off-chain clients use ──
const (
	TxCreatePayLink    = types.TxCreatePayLink
	TxSubmitValidation = types.TxSubmitValidation
	TxStake            = types.TxStake
)

// ── Hex / byte helpers (thin re-exports) ──

func HexToHash(s string) Hash       { return types.HexToHash(s) }
func HexToAddress(s string) Address { return types.HexToAddress(s) }
func BytesToHash(b []byte) Hash     { return types.BytesToHash(b) }
func BytesToAddress(b []byte) Address {
	return types.BytesToAddress(b)
}

// ProofHash is THE canonical on-chain proof identity used for anti-replay (invariant A.7) and
// per-PayLink proof-hash consistency. It binds the PayLink id, the rail transaction id, and the
// amount, so a different rail tx or a different amount yields a different hash. It MUST be computed
// identically by every component that settles or checks a proof (the proof-validator and every
// adapter, work04). Definition:
//
//	ProofHash = SHA256( go_json({"paylinkId": <plID 0x-hex>, "txId": <txID>, "amount": <amount>}) )
//
// where go_json is encoding/json.Marshal of the struct below (compact, fields in declared order,
// HTML-escaped) — the same JSON convention the chain uses for Transaction.SignableBytes.
func ProofHash(plID Hash, txID string, amount uint64) Hash {
	b, _ := json.Marshal(struct {
		PayLinkID Hash   `json:"paylinkId"`
		TxID      string `json:"txId"`
		Amount    uint64 `json:"amount"`
	}{PayLinkID: plID, TxID: txID, Amount: amount})
	return crypto.SHA256Hash(b)
}

// buildTx assembles an UNSIGNED transaction: Type/From/Nonce set, Payload marshaled, and
// Signature/Hash left zero for SignTx to fill. The marshaled payload is what SignableBytes embeds,
// so it must be produced here (not by the caller) to keep the wire bytes canonical.
func buildTx(txType TxType, from Address, nonce uint64, payload any) (*Transaction, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s payload: %w", txType, err)
	}
	return &Transaction{Type: txType, From: from, Nonce: nonce, Payload: raw}, nil
}

// BuildSubmitValidationTx assembles an unsigned TxSubmitValidation that settles plID with the given
// proofHash. Sign it with SignTx before broadcasting.
func BuildSubmitValidationTx(from Address, nonce uint64, plID, proofHash Hash) (*Transaction, error) {
	return buildTx(TxSubmitValidation, from, nonce, SubmitValidationPayload{PayLinkID: plID, ProofHash: proofHash})
}

// BuildStakeTx assembles an unsigned TxStake for the given amount (used by the devnet auto-stake
// bootstrap so the validator becomes active). Sign it with SignTx before broadcasting.
func BuildStakeTx(from Address, nonce uint64, amount uint64) (*Transaction, error) {
	return buildTx(TxStake, from, nonce, StakePayload{Amount: amount})
}

// BuildCreatePayLinkTx assembles an unsigned TxCreatePayLink. Primarily for clients/tests that need
// to create a PayLink directly on-chain. Sign it with SignTx before broadcasting.
func BuildCreatePayLinkTx(from Address, nonce uint64, p CreatePayLinkPayload) (*Transaction, error) {
	return buildTx(TxCreatePayLink, from, nonce, p)
}
