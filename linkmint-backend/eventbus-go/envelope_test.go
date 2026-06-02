package eventbus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// goldenEnvelope mirrors testdata/envelope.golden.json. The payload is built from a struct whose
// field order is deliberately NOT alphabetical (pl_id, amount, currency) to prove the canonicalizer
// sorts keys regardless of input order.
func goldenEnvelope(t *testing.T) Envelope {
	t.Helper()
	payload, err := canonicalPayload(struct {
		PlID     string `json:"pl_id"`
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}{PlID: "PLK_demo", Amount: "1000", Currency: "KES"})
	if err != nil {
		t.Fatalf("canonicalPayload: %v", err)
	}
	return Envelope{
		ID:            "00000000-0000-0000-0000-000000000001",
		Name:          "paylink.verified",
		Key:           "PLK_demo",
		CorrelationID: "trace-abc",
		OccurredAt:    "2026-06-01T12:00:00Z",
		Source:        "paylink-service",
		Payload:       payload,
	}
}

func TestMarshal_MatchesGolden(t *testing.T) {
	got, err := goldenEnvelope(t).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "envelope.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(trimNL(want)) {
		t.Fatalf("envelope bytes != golden\n got: %s\nwant: %s", got, trimNL(want))
	}
}

func TestCanonicalPayload_SortsKeysRecursively(t *testing.T) {
	got, err := canonicalPayload(map[string]any{
		"z": "1",
		"a": map[string]any{"y": "2", "x": "3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"a":{"x":"3","y":"2"},"z":"1"}`; string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCanonicalPayload_PreservesLargeIntegersExactly(t *testing.T) {
	// A float64 round-trip would corrupt this 17-digit integer; UseNumber must preserve it.
	got, err := canonicalPayload(map[string]any{"n": int64(12345678901234567)})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"n":12345678901234567}`; string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCanonicalPayload_DoesNotHTMLEscape(t *testing.T) {
	got, err := canonicalPayload(map[string]any{"note": "a<b&c>d"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"note":"a<b&c>d"}`; string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestMarshalUnmarshal_RoundTrip(t *testing.T) {
	orig := goldenEnvelope(t)
	b, err := orig.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalEnvelope(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != orig.Name || got.Key != orig.Key || got.Source != orig.Source {
		t.Fatalf("header mismatch: %+v", got)
	}
	if string(got.Payload) != string(orig.Payload) {
		t.Fatalf("payload mismatch: %s != %s", got.Payload, orig.Payload)
	}
}

func TestUnmarshalEnvelope_IgnoresUnknownFields(t *testing.T) {
	b := []byte(`{"id":"x","name":"paylink.verified","extra":"ignored","payload":{"a":1}}`)
	got, err := UnmarshalEnvelope(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "paylink.verified" || string(got.Payload) != `{"a":1}` {
		t.Fatalf("got %+v", got)
	}
}

func TestNewEnvelope_StampsIDAndTimestamp(t *testing.T) {
	e, err := NewEnvelope("payment.failed", "PMT_1", "trace-1", "payment-orchestrator", map[string]any{"reason": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if e.ID == "" {
		t.Error("expected a generated id")
	}
	if len(e.OccurredAt) != len("2006-01-02T15:04:05Z") || e.OccurredAt[len(e.OccurredAt)-1] != 'Z' {
		t.Errorf("occurred_at not RFC3339-Z seconds: %q", e.OccurredAt)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(e.Payload, &probe); err != nil {
		t.Fatalf("payload not valid json: %v", err)
	}
}

func trimNL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
