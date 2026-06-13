// Package ingest parses rail settlement files into the rail-agnostic line shape the store matches
// against payouts. Two formats are supported (auto-detected by the leading byte):
//
//	JSON  {"rail":"mpesa","lines":[{"reference":"PO-...","amount":"1490","currency":"KES"},...]}
//	CSV   header "reference,amount,currency" followed by one matching line per row
//
// The 3-way reconciliation algorithm itself is work27; this only normalizes the file.
package ingest

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/paylink/settlement-service/internal/domain"
)

// File is the parsed rail settlement file.
type File struct {
	Rail  string
	Lines []domain.RailFileLine
}

type jsonFile struct {
	Rail  string `json:"rail"`
	Lines []struct {
		Reference string `json:"reference"`
		Amount    string `json:"amount"`
		Currency  string `json:"currency"`
	} `json:"lines"`
}

// Parse reads a rail settlement file (JSON or CSV) and returns its normalized lines. railHint sets
// the rail when the file format does not carry one (CSV always; JSON when "rail" is omitted).
func Parse(body []byte, railHint string) (File, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return File{}, fmt.Errorf("empty file")
	}
	if trimmed[0] == '{' {
		return parseJSON(trimmed, railHint)
	}
	return parseCSV(trimmed, railHint)
}

func parseJSON(body []byte, railHint string) (File, error) {
	var jf jsonFile
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&jf); err != nil {
		return File{}, fmt.Errorf("invalid JSON rail file: %w", err)
	}
	rail := jf.Rail
	if rail == "" {
		rail = railHint
	}
	out := File{Rail: rail}
	for i, l := range jf.Lines {
		line, err := newLine(l.Reference, l.Amount, l.Currency)
		if err != nil {
			return File{}, fmt.Errorf("line %d: %w", i+1, err)
		}
		out.Lines = append(out.Lines, line)
	}
	return out, nil
}

func parseCSV(body []byte, railHint string) (File, error) {
	r := csv.NewReader(bytes.NewReader(body))
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return File{}, fmt.Errorf("invalid CSV rail file: %w", err)
	}
	ref, amt, ccy := columnIndexes(header)
	if ref < 0 || amt < 0 || ccy < 0 {
		return File{}, fmt.Errorf("CSV header must contain reference, amount, currency")
	}
	out := File{Rail: railHint}
	row := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return File{}, fmt.Errorf("row %d: %w", row, err)
		}
		row++
		if len(rec) <= max3(ref, amt, ccy) {
			return File{}, fmt.Errorf("row %d: too few columns", row)
		}
		line, err := newLine(rec[ref], rec[amt], rec[ccy])
		if err != nil {
			return File{}, fmt.Errorf("row %d: %w", row, err)
		}
		out.Lines = append(out.Lines, line)
	}
	return out, nil
}

func newLine(ref, amount, currency string) (domain.RailFileLine, error) {
	ref = strings.TrimSpace(ref)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if ref == "" || currency == "" || !ok || amt.Sign() < 0 {
		return domain.RailFileLine{}, fmt.Errorf("reference, amount (non-negative integer minor units), and currency are required")
	}
	return domain.RailFileLine{Reference: ref, Amount: amt, AmountStr: amt.String(), Currency: currency}, nil
}

func columnIndexes(header []string) (ref, amt, ccy int) {
	ref, amt, ccy = -1, -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "reference", "ref":
			ref = i
		case "amount", "amt":
			amt = i
		case "currency", "ccy":
			ccy = i
		}
	}
	return ref, amt, ccy
}

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
