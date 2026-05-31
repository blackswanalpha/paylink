// Package proof builds the rail-agnostic payment proof (invariant A.4) the adapter broadcasts to
// the proof-validator (work03), and the canonical bytes it signs.
//
// IMPORTANT: CanonicalBytes MUST reproduce the proof-validator's bytes byte-for-byte
// (linkmint-backend/proof-validator/internal/proof/proof.go). That package lives in another
// module's internal/ tree and cannot be imported here, so the contract is duplicated and locked by
// a golden-bytes test (proof_test.go) asserting the exact JSON string. The on-chain identity
// (lvm.ProofHash) is computed by the validator/chain, not here.
package proof

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Proof is the normalized payment proof. Amount is in minor units and matches the on-chain PayLink
// amount (uint64). Sender/Receiver are rail identifiers (payer MSISDN / receiver shortcode), not
// on-chain addresses. Signature is base64(P-256 r||s, 64 bytes) over CanonicalBytes.
type Proof struct {
	PayLinkID string
	Rail      string
	TxID      string
	Amount    uint64
	Timestamp int64
	Sender    string
	Receiver  string
	Signature string
}

// wireProof is the exact JSON body POSTed to the validator's POST /v1/proofs (amount as a number;
// the validator accepts number or string). Field tags match the proof contract.
type wireProof struct {
	PayLinkID string `json:"pl_id"`
	Rail      string `json:"rail"`
	TxID      string `json:"tx_id"`
	Amount    uint64 `json:"amount"`
	Timestamp int64  `json:"timestamp"`
	Sender    string `json:"sender"`
	Receiver  string `json:"receiver"`
	Signature string `json:"proof_signature"`
}

// MarshalWire returns the JSON body to POST to the proof-validator (includes proof_signature).
func MarshalWire(p Proof) ([]byte, error) {
	return json.Marshal(wireProof{
		PayLinkID: p.PayLinkID,
		Rail:      p.Rail,
		TxID:      p.TxID,
		Amount:    p.Amount,
		Timestamp: p.Timestamp,
		Sender:    p.Sender,
		Receiver:  p.Receiver,
		Signature: p.Signature,
	})
}

// CanonicalBytes are the exact bytes the adapter signs to produce proof_signature: the compact,
// HTML-escaped JSON of the proof WITHOUT the signature, fields in this fixed order. These MUST be
// byte-for-byte identical to the proof-validator's CanonicalBytes. (See package doc + proof_test.go.)
func CanonicalBytes(p Proof) []byte {
	b, _ := json.Marshal(struct {
		PayLinkID string `json:"pl_id"`
		Rail      string `json:"rail"`
		TxID      string `json:"tx_id"`
		Amount    uint64 `json:"amount"`
		Timestamp int64  `json:"timestamp"`
		Sender    string `json:"sender"`
		Receiver  string `json:"receiver"`
	}{p.PayLinkID, p.Rail, p.TxID, p.Amount, p.Timestamp, p.Sender, p.Receiver})
	return b
}

// Validate is a defensive guard run before signing/broadcasting: a malformed proof here is an
// internal bug in normalization (not a client error). It mirrors the validator's shape gate so the
// adapter never broadcasts something the validator would reject.
func Validate(p Proof) error {
	if !isHash(p.PayLinkID) {
		return fmt.Errorf("pl_id must be a 0x-prefixed 32-byte hex hash, got %q", p.PayLinkID)
	}
	if p.Rail != "mpesa" {
		return fmt.Errorf("rail must be \"mpesa\", got %q", p.Rail)
	}
	if p.TxID == "" {
		return fmt.Errorf("tx_id is required")
	}
	if p.Amount == 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if p.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be a positive unix time")
	}
	if p.Sender == "" {
		return fmt.Errorf("sender is required")
	}
	if p.Receiver == "" {
		return fmt.Errorf("receiver is required")
	}
	return nil
}

// isHash reports whether s is a 0x-prefixed 32-byte (64 hex char) hash.
func isHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	b, err := hex.DecodeString(s[2:])
	return err == nil && len(b) == 32
}
