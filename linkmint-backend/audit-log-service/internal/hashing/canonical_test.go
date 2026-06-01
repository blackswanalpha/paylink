package hashing

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func sample() Input {
	return Input{
		OccurredAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		ActorID:    "11111111-1111-1111-1111-111111111111",
		ActorKind:  "user",
		Action:     "admin.search",
		Resource:   "user:abc",
		Context:    json.RawMessage(`{"trace_id":"t1","ip":"1.2.3.4"}`),
	}
}

func TestCanonicalDeterministic(t *testing.T) {
	in := sample()
	a, err := Canonical(in)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	b, _ := Canonical(in)
	if !bytes.Equal(a, b) {
		t.Fatalf("canonical not deterministic:\n%s\n%s", a, b)
	}
}

func TestObjectKeyOrderIndependent(t *testing.T) {
	in1 := sample()
	in1.Context = json.RawMessage(`{"a":1,"b":2,"z":{"x":1,"y":2}}`)
	in2 := sample()
	in2.Context = json.RawMessage(`{"b":2,"a":1,"z":{"y":2,"x":1}}`)
	h1, _ := EntryHash(Genesis(), in1)
	h2, _ := EntryHash(Genesis(), in2)
	if !bytes.Equal(h1, h2) {
		t.Fatal("object key order must not change the hash")
	}
}

func TestArrayOrderMatters(t *testing.T) {
	in1 := sample()
	in1.Context = json.RawMessage(`{"k":[1,2,3]}`)
	in2 := sample()
	in2.Context = json.RawMessage(`{"k":[3,2,1]}`)
	h1, _ := EntryHash(Genesis(), in1)
	h2, _ := EntryHash(Genesis(), in2)
	if bytes.Equal(h1, h2) {
		t.Fatal("array element order must change the hash")
	}
}

func TestLargeIntegerLexemePreserved(t *testing.T) {
	// Beyond float64's exact integer range: a float64 round-trip would corrupt this. UseNumber keeps
	// the lexeme, so the canonical bytes contain the exact digits and re-hashing is stable.
	in := sample()
	in.Context = json.RawMessage(`{"n":900719925474099123}`)
	c, err := Canonical(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(c, []byte("900719925474099123")) {
		t.Fatalf("large integer lexeme not preserved: %s", c)
	}
}

func TestNullVsAbsentHashEqual(t *testing.T) {
	in1 := sample()
	in1.Before = nil
	in2 := sample()
	in2.Before = json.RawMessage(`null`)
	in3 := sample()
	in3.Before = json.RawMessage(`  null `)
	h1, _ := EntryHash(Genesis(), in1)
	h2, _ := EntryHash(Genesis(), in2)
	h3, _ := EntryHash(Genesis(), in3)
	if !bytes.Equal(h1, h2) || !bytes.Equal(h1, h3) {
		t.Fatal("absent / explicit null / whitespace-null before must hash identically")
	}
}

func TestTimestampNormalizedToUTC(t *testing.T) {
	loc := time.FixedZone("KE", 3*3600)
	in1 := sample()
	in1.OccurredAt = time.Date(2026, 6, 1, 15, 0, 0, 0, loc) // 12:00Z
	in2 := sample()
	in2.OccurredAt = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	h1, _ := EntryHash(Genesis(), in1)
	h2, _ := EntryHash(Genesis(), in2)
	if !bytes.Equal(h1, h2) {
		t.Fatal("the same instant in different zones must hash identically")
	}
}

func TestTimestampMicrosecondTruncation(t *testing.T) {
	in1 := sample()
	in1.OccurredAt = time.Date(2026, 6, 1, 12, 0, 0, 123456789, time.UTC) // ns
	in2 := sample()
	in2.OccurredAt = time.Date(2026, 6, 1, 12, 0, 0, 123456000, time.UTC) // us
	h1, _ := EntryHash(Genesis(), in1)
	h2, _ := EntryHash(Genesis(), in2)
	if !bytes.Equal(h1, h2) {
		t.Fatal("sub-microsecond precision must be truncated (timestamptz round-trip)")
	}
}

func TestPrevHashChangesEntryHash(t *testing.T) {
	in := sample()
	h1, err := EntryHash(Genesis(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(h1) != HashLen {
		t.Fatalf("hash length = %d, want %d", len(h1), HashLen)
	}
	prev2 := make([]byte, HashLen)
	prev2[0] = 1
	h2, _ := EntryHash(prev2, in)
	if bytes.Equal(h1, h2) {
		t.Fatal("changing prev_hash must change entry_hash")
	}
}

func TestActorIDNullWhenEmpty(t *testing.T) {
	in := sample()
	in.ActorID = ""
	c, err := Canonical(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(c, []byte(`"actor":{"id":null,`)) {
		t.Fatalf("empty actor id should canonicalize to null: %s", c)
	}
}

func TestInvalidJSONErrors(t *testing.T) {
	in := sample()
	in.Context = json.RawMessage(`{not json`)
	if _, err := Canonical(in); err == nil {
		t.Fatal("expected an error for invalid context JSON")
	}
}

func TestGenesisIsZero(t *testing.T) {
	g := Genesis()
	if len(g) != HashLen {
		t.Fatalf("genesis length = %d", len(g))
	}
	for _, b := range g {
		if b != 0 {
			t.Fatal("genesis must be all zero bytes")
		}
	}
}
