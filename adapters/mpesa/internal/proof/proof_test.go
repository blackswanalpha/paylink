package proof_test

import (
	"strings"
	"testing"

	"github.com/paylink/mpesa-adapter/internal/proof"
)

// valid is a representative normalized proof (signature filled in by tests that need it).
func valid() proof.Proof {
	return proof.Proof{
		PayLinkID: "0x" + strings.Repeat("ab", 32),
		Rail:      "mpesa",
		TxID:      "MP-1",
		Amount:    1500,
		Timestamp: 1730000000,
		Sender:    "254700000000",
		Receiver:  "254711111111",
	}
}

// TestCanonicalBytes locks the byte-exact contract with the proof-validator. The expected string is
// copied verbatim from proof-validator/internal/proof/proof_test.go (TestCanonicalBytes). If this
// drifts, the validator will reject every signature.
func TestCanonicalBytes(t *testing.T) {
	p := valid()
	want := `{"pl_id":"0x` + strings.Repeat("ab", 32) + `","rail":"mpesa","tx_id":"MP-1","amount":1500,"timestamp":1730000000,"sender":"254700000000","receiver":"254711111111"}`
	got := string(proof.CanonicalBytes(p))
	if got != want {
		t.Fatalf("canonical bytes:\n got %s\nwant %s", got, want)
	}
	if strings.Contains(got, "proof_signature") {
		t.Fatal("canonical bytes must not include proof_signature")
	}
	if string(proof.CanonicalBytes(p)) != got {
		t.Fatal("canonical bytes not deterministic")
	}
}

// TestCanonicalBytes_HTMLEscaped mirrors the validator: encoding/json HTML-escapes & < > so the
// adapter reproduces the same bytes.
func TestCanonicalBytes_HTMLEscaped(t *testing.T) {
	p := valid()
	p.Sender = "a&b<c>"
	got := string(proof.CanonicalBytes(p))
	for _, raw := range []string{"&", "<", ">"} {
		if strings.Contains(got, raw) {
			t.Fatalf("expected %q to be HTML-escaped, got %s", raw, got)
		}
	}
}

func TestValidate_Valid(t *testing.T) {
	if err := proof.Validate(valid()); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
}

func TestValidate_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*proof.Proof)
	}{
		{"short pl_id", func(p *proof.Proof) { p.PayLinkID = "0xabc" }},
		{"no 0x pl_id", func(p *proof.Proof) { p.PayLinkID = strings.Repeat("ab", 32) }},
		{"non-hex pl_id", func(p *proof.Proof) { p.PayLinkID = "0x" + strings.Repeat("zz", 32) }},
		{"wrong rail", func(p *proof.Proof) { p.Rail = "card" }},
		{"empty tx_id", func(p *proof.Proof) { p.TxID = "" }},
		{"zero amount", func(p *proof.Proof) { p.Amount = 0 }},
		{"zero timestamp", func(p *proof.Proof) { p.Timestamp = 0 }},
		{"empty sender", func(p *proof.Proof) { p.Sender = "" }},
		{"empty receiver", func(p *proof.Proof) { p.Receiver = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(&p)
			if err := proof.Validate(p); err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
		})
	}
}

// TestMarshalWire confirms the broadcast body carries the proof_signature and all eight fields.
func TestMarshalWire(t *testing.T) {
	p := valid()
	p.Signature = "c2ln" // "sig"
	b, err := proof.MarshalWire(p)
	if err != nil {
		t.Fatalf("MarshalWire: %v", err)
	}
	for _, f := range []string{`"pl_id"`, `"rail"`, `"tx_id"`, `"amount"`, `"timestamp"`, `"sender"`, `"receiver"`, `"proof_signature"`} {
		if !strings.Contains(string(b), f) {
			t.Fatalf("wire body missing %s: %s", f, b)
		}
	}
}
