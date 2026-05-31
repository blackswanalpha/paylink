// Package proof defines the rail-agnostic payment proof (invariant A.4), its strict decoding,
// shape validation, and the canonical bytes an adapter signs. The proof's `proof_signature` is an
// off-chain trust contract between adapters (work04) and this validator; the chain never sees it —
// only the derived on-chain proofHash (see lvm.ProofHash). Verification lives in verifier.go.
package proof

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/paylink/proof-validator/internal/httpx"
)

// rails are the allowed rail labels (the only rail-specific value that crosses the boundary).
var rails = map[string]struct{}{"mpesa": {}, "card": {}, "bank": {}, "crypto": {}}

// Proof is the normalized payment proof an adapter submits. Amount is in minor units and matches
// the on-chain PayLink amount (uint64).
type Proof struct {
	PayLinkID string
	Rail      string
	TxID      string
	Amount    uint64
	Timestamp int64
	Sender    string
	Receiver  string
	Signature string // base64(P-256 r||s, 64 bytes)
}

// flexUint64 decodes a uint64 from either a JSON number (1500) or a JSON string ("1500"), so
// adapters may send `amount` either way. CanonicalBytes/lvm.ProofHash always use the uint64.
type flexUint64 uint64

func (a *flexUint64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return errors.New("amount is required")
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("amount must be a non-negative integer, got %q", s)
	}
	*a = flexUint64(n)
	return nil
}

type wireProof struct {
	PayLinkID string     `json:"pl_id"`
	Rail      string     `json:"rail"`
	TxID      string     `json:"tx_id"`
	Amount    flexUint64 `json:"amount"`
	Timestamp int64      `json:"timestamp"`
	Sender    string     `json:"sender"`
	Receiver  string     `json:"receiver"`
	Signature string     `json:"proof_signature"`
}

// Parse strictly decodes the request body into a Proof (rejecting unknown fields and trailing data
// as INVALID_PAYLOAD). It does not validate semantics — call ValidateShape next.
func Parse(raw []byte) (Proof, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var wp wireProof
	if err := dec.Decode(&wp); err != nil {
		return Proof{}, httpx.NewError(httpx.CodeInvalidPayload, "request body is not valid JSON: "+err.Error(), nil)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Proof{}, httpx.NewError(httpx.CodeInvalidPayload, "request body must contain a single JSON object", nil)
	}
	return Proof{
		PayLinkID: wp.PayLinkID,
		Rail:      wp.Rail,
		TxID:      wp.TxID,
		Amount:    uint64(wp.Amount),
		Timestamp: wp.Timestamp,
		Sender:    wp.Sender,
		Receiver:  wp.Receiver,
		Signature: wp.Signature,
	}, nil
}

func shapeErr(msg string, details map[string]any) error {
	return httpx.NewError(httpx.CodeInvalidProofShape, msg, details)
}

// ValidateShape checks the proof is structurally well-formed. It is the A.4 gate: only the
// normalized shape is accepted, and nothing is broadcast unless it passes.
func ValidateShape(p Proof) error {
	if !isHash(p.PayLinkID) {
		return shapeErr("pl_id must be a 0x-prefixed 32-byte hex hash", map[string]any{"pl_id": p.PayLinkID})
	}
	if _, ok := rails[p.Rail]; !ok {
		return shapeErr("rail must be one of mpesa|card|bank|crypto", map[string]any{"rail": p.Rail})
	}
	if p.TxID == "" {
		return shapeErr("tx_id is required", nil)
	}
	if p.Amount == 0 {
		return shapeErr("amount must be greater than zero", nil)
	}
	if p.Timestamp <= 0 {
		return shapeErr("timestamp must be a positive unix time", nil)
	}
	if p.Sender == "" {
		return shapeErr("sender is required", nil)
	}
	if p.Receiver == "" {
		return shapeErr("receiver is required", nil)
	}
	if sig, err := base64.StdEncoding.DecodeString(p.Signature); err != nil || len(sig) != 64 {
		return shapeErr("proof_signature must be base64 of a 64-byte r||s signature", nil)
	}
	return nil
}

// CanonicalBytes are the exact bytes an adapter signs to produce proof_signature: the compact,
// HTML-escaped JSON of the proof WITHOUT the signature, fields in this fixed order. work04's
// adapter MUST reproduce these byte-for-byte. (See DESIGN.md.)
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

// isHash reports whether s is a 0x-prefixed 32-byte (64 hex char) hash.
func isHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	b, err := hex.DecodeString(s[2:])
	return err == nil && len(b) == 32
}
