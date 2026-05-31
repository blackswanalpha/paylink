package proof_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/paylink/proof-validator/internal/httpx"
	"github.com/paylink/proof-validator/internal/proof"
)

func valid() proof.Proof {
	return proof.Proof{
		PayLinkID: "0x" + strings.Repeat("ab", 32),
		Rail:      "mpesa",
		TxID:      "MP-1",
		Amount:    1500,
		Timestamp: 1730000000,
		Sender:    "254700000000",
		Receiver:  "254711111111",
		// base64 of 64 zero bytes
		Signature: strings.Repeat("A", 86) + "==",
	}
}

func code(t *testing.T, err error) httpx.ErrorCode {
	t.Helper()
	var ae *httpx.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not an AppError", err)
	}
	return ae.Code
}

func TestValidateShape_Valid(t *testing.T) {
	if err := proof.ValidateShape(valid()); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
}

func TestValidateShape_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*proof.Proof)
	}{
		{"short pl_id", func(p *proof.Proof) { p.PayLinkID = "0xabc" }},
		{"no 0x pl_id", func(p *proof.Proof) { p.PayLinkID = strings.Repeat("ab", 32) }},
		{"non-hex pl_id", func(p *proof.Proof) { p.PayLinkID = "0x" + strings.Repeat("zz", 32) }},
		{"bad rail", func(p *proof.Proof) { p.Rail = "swift" }},
		{"empty tx_id", func(p *proof.Proof) { p.TxID = "" }},
		{"zero amount", func(p *proof.Proof) { p.Amount = 0 }},
		{"zero timestamp", func(p *proof.Proof) { p.Timestamp = 0 }},
		{"empty sender", func(p *proof.Proof) { p.Sender = "" }},
		{"empty receiver", func(p *proof.Proof) { p.Receiver = "" }},
		{"non-base64 sig", func(p *proof.Proof) { p.Signature = "!!!notbase64!!!" }},
		{"short sig", func(p *proof.Proof) { p.Signature = "AAAA" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(&p)
			err := proof.ValidateShape(p)
			if err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
			if c := code(t, err); c != httpx.CodeInvalidProofShape {
				t.Fatalf("code = %s, want INVALID_PROOF_SHAPE", c)
			}
		})
	}
}

func TestParse_AmountNumberOrString(t *testing.T) {
	base := `{"pl_id":"0x` + strings.Repeat("ab", 32) + `","rail":"mpesa","tx_id":"t","timestamp":1,"sender":"s","receiver":"r","proof_signature":"x",`
	for _, amt := range []string{`"amount":1500}`, `"amount":"1500"}`} {
		p, err := proof.Parse([]byte(base + amt))
		if err != nil {
			t.Fatalf("Parse(%s): %v", amt, err)
		}
		if p.Amount != 1500 {
			t.Fatalf("amount = %d, want 1500 (from %s)", p.Amount, amt)
		}
	}
}

func TestParse_Rejects(t *testing.T) {
	cases := map[string]string{
		"unknown field": `{"pl_id":"0x","extra":1}`,
		"malformed":     `{`,
		"trailing data": `{"pl_id":"0x"}{}`,
		"bad amount":    `{"amount":"not-a-number"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := proof.Parse([]byte(body)); err == nil {
				t.Fatalf("expected Parse error for %s", name)
			} else if c := code(t, err); c != httpx.CodeInvalidPayload {
				t.Fatalf("code = %s, want INVALID_PAYLOAD", c)
			}
		})
	}
}

func TestCanonicalBytes(t *testing.T) {
	p := valid()
	want := `{"pl_id":"0x` + strings.Repeat("ab", 32) + `","rail":"mpesa","tx_id":"MP-1","amount":1500,"timestamp":1730000000,"sender":"254700000000","receiver":"254711111111"}`
	got := string(proof.CanonicalBytes(p))
	if got != want {
		t.Fatalf("canonical bytes:\n got %s\nwant %s", got, want)
	}
	// Excludes the signature and is deterministic.
	if strings.Contains(got, "proof_signature") {
		t.Fatal("canonical bytes must not include proof_signature")
	}
	if string(proof.CanonicalBytes(p)) != got {
		t.Fatal("canonical bytes not deterministic")
	}
}

func TestCanonicalBytes_HTMLEscaped(t *testing.T) {
	p := valid()
	p.Sender = "a&b<c>"
	got := string(proof.CanonicalBytes(p))
	// Go's encoding/json HTML-escapes & < > (to & < >) — the same convention the
	// chain's SignableBytes uses, so adapters reproducing these bytes must escape identically.
	// The escaped output therefore contains NONE of the raw characters.
	for _, raw := range []string{"&", "<", ">"} {
		if strings.Contains(got, raw) {
			t.Fatalf("expected %q to be HTML-escaped, got %s", raw, got)
		}
	}
}
