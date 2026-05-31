package state

import (
	"testing"

	"github.com/paylink/paylink-chain/internal/types"
)

func TestMintTokens_RespectsMaxSupplyCap(t *testing.T) {
	to := types.HexToAddress("0x0000000000000000000000000000000000000099")
	genesis := &types.GenesisConfig{
		ChainID:       "test",
		AdminAddress:  types.HexToAddress("0x01"),
		InitialSupply: 900,
		MaxSupply:     1000,
		InitialBalances: []types.GenesisBalance{
			{Address: types.HexToAddress("0x01"), Balance: 900},
		},
	}
	s := NewStateDB(genesis)

	if err := s.MintTokens(to, 100); err != nil {
		t.Fatalf("mint up to cap should succeed: %v", err)
	}
	if got := s.TotalSupply(); got != 1000 {
		t.Fatalf("supply at cap = %d, want 1000", got)
	}

	if err := s.MintTokens(to, 1); err == nil {
		t.Fatalf("mint past cap should fail")
	}
	if got := s.TotalSupply(); got != 1000 {
		t.Fatalf("supply after rejected mint = %d, want 1000 (unchanged)", got)
	}
	if got := s.GetBalance(to); got != 100 {
		t.Fatalf("recipient balance after rejected mint = %d, want 100 (unchanged)", got)
	}
}
