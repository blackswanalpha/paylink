package ingest

import "testing"

func TestParseJSON(t *testing.T) {
	body := []byte(`{"rail":"mpesa","lines":[
		{"reference":"PO-1","amount":"1500","currency":"KES"},
		{"reference":"PO-2","amount":"500","currency":"kes"}]}`)
	f, err := Parse(body, "ignored")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Rail != "mpesa" || len(f.Lines) != 2 {
		t.Fatalf("rail=%q lines=%d", f.Rail, len(f.Lines))
	}
	if f.Lines[0].Amount.String() != "1500" || f.Lines[1].Currency != "KES" {
		t.Fatalf("unexpected line parse: %+v", f.Lines)
	}
}

func TestParseJSONRailHint(t *testing.T) {
	f, err := Parse([]byte(`{"lines":[{"reference":"PO-1","amount":"1","currency":"KES"}]}`), "swift")
	if err != nil {
		t.Fatal(err)
	}
	if f.Rail != "swift" {
		t.Fatalf("rail=%q, want swift (from hint)", f.Rail)
	}
}

func TestParseCSV(t *testing.T) {
	body := []byte("reference,amount,currency\nPO-1,1500,KES\nPO-2,500,KES\n")
	f, err := Parse(body, "mpesa")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Rail != "mpesa" || len(f.Lines) != 2 {
		t.Fatalf("rail=%q lines=%d", f.Rail, len(f.Lines))
	}
	if f.Lines[1].Reference != "PO-2" || f.Lines[1].Amount.String() != "500" {
		t.Fatalf("unexpected line: %+v", f.Lines[1])
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string][]byte{
		"empty":          []byte("   "),
		"bad json":       []byte(`{"lines": [`),
		"bad amount":     []byte(`{"lines":[{"reference":"PO-1","amount":"x","currency":"KES"}]}`),
		"missing fields": []byte(`{"lines":[{"reference":"","amount":"1","currency":"KES"}]}`),
		"csv no header":  []byte("PO-1,1500,KES\n"),
	}
	for name, body := range cases {
		if _, err := Parse(body, "mpesa"); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
