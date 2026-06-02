package ledger

import (
	"errors"
	"math/big"
	"testing"
)

func leg(account string, dir Direction, amount int64, ccy string) Leg {
	return Leg{Account: account, Direction: dir, Amount: big.NewInt(amount), Currency: ccy}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		entries []Leg
		wantErr error // nil | ErrUnbalanced | ErrInvalidLeg
	}{
		{
			name:    "balanced single currency",
			entries: []Leg{leg("paylink:PLK1", DR, 100, "PLN"), leg("treasury", CR, 100, "PLN")},
		},
		{
			name: "balanced fee split 70/20/10",
			entries: []Leg{
				leg("paylink:PLK1", DR, 1000, "PLN"),
				leg("validator:0xabc", CR, 700, "PLN"),
				leg("treasury", CR, 200, "PLN"),
				leg("burn", CR, 100, "PLN"),
			},
		},
		{
			name: "multi-currency balanced per currency",
			entries: []Leg{
				leg("a", DR, 100, "PLN"), leg("b", CR, 100, "PLN"),
				leg("c", DR, 50, "USD"), leg("d", CR, 50, "USD"),
			},
		},
		{
			name:    "unbalanced amounts",
			entries: []Leg{leg("a", DR, 100, "PLN"), leg("b", CR, 90, "PLN")},
			wantErr: ErrUnbalanced,
		},
		{
			name: "multi-currency one side unbalanced",
			entries: []Leg{
				leg("a", DR, 100, "PLN"), leg("b", CR, 100, "PLN"),
				leg("c", DR, 50, "USD"), leg("d", CR, 40, "USD"),
			},
			wantErr: ErrUnbalanced,
		},
		{
			name:    "empty",
			entries: nil,
			wantErr: ErrUnbalanced,
		},
		{
			name:    "single leg",
			entries: []Leg{leg("a", DR, 100, "PLN")},
			wantErr: ErrUnbalanced,
		},
		{
			name:    "all DR no CR",
			entries: []Leg{leg("a", DR, 50, "PLN"), leg("b", DR, 50, "PLN")},
			wantErr: ErrUnbalanced,
		},
		{
			name:    "bad direction",
			entries: []Leg{{Account: "a", Direction: "XX", Amount: big.NewInt(100), Currency: "PLN"}, leg("b", CR, 100, "PLN")},
			wantErr: ErrInvalidLeg,
		},
		{
			name:    "zero amount",
			entries: []Leg{leg("a", DR, 0, "PLN"), leg("b", CR, 0, "PLN")},
			wantErr: ErrInvalidLeg,
		},
		{
			name:    "negative amount",
			entries: []Leg{leg("a", DR, -5, "PLN"), leg("b", CR, -5, "PLN")},
			wantErr: ErrInvalidLeg,
		},
		{
			name:    "nil amount",
			entries: []Leg{{Account: "a", Direction: DR, Amount: nil, Currency: "PLN"}, leg("b", CR, 100, "PLN")},
			wantErr: ErrInvalidLeg,
		},
		{
			name:    "empty account",
			entries: []Leg{leg("  ", DR, 100, "PLN"), leg("b", CR, 100, "PLN")},
			wantErr: ErrInvalidLeg,
		},
		{
			name:    "empty currency",
			entries: []Leg{leg("a", DR, 100, ""), leg("b", CR, 100, "")},
			wantErr: ErrInvalidLeg,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.entries)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validate() = %v, want errors.Is %v", err, tt.wantErr)
			}
		})
	}
}

func TestBigAmountStringRoundTrip(t *testing.T) {
	// A 38-digit value (the NUMERIC(38,0) ceiling) exceeds int64; *big.Int + string round-trip is exact.
	huge, ok := new(big.Int).SetString("99999999999999999999999999999999999999", 10)
	if !ok {
		t.Fatal("setup: bad literal")
	}
	back, ok := new(big.Int).SetString(huge.String(), 10)
	if !ok || back.Cmp(huge) != 0 {
		t.Fatalf("round-trip failed for %s", huge)
	}
}
