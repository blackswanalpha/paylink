package fee

import (
	"fmt"
	"testing"

	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

func setupDistributorTest(t *testing.T, numValidators int) (*state.StateDB, *Distributor, []types.Address) {
	t.Helper()

	treasury := types.HexToAddress("0xTREASURY000000000000000000000000000000")
	genesis := &types.GenesisConfig{
		ChainID:         "test",
		AdminAddress:    types.HexToAddress("0x01"),
		MaxSupply:       1_000_000_000,
		MinimumStake:    100,
		TreasuryAddress: treasury,
	}
	s := state.NewStateDB(genesis)

	var validators []types.Address
	for i := 0; i < numValidators; i++ {
		addr := types.HexToAddress(fmt.Sprintf("0x%040x", i+200))
		s.AddBalance(addr, 10000)
		if err := s.Stake(addr, 10000, 1000); err != nil {
			t.Fatal(err)
		}
		validators = append(validators, addr)
	}

	d := NewDistributor(s, treasury)
	return s, d, validators
}

func TestDistributor_EqualStakeDistribution(t *testing.T) {
	s, d, validators := setupDistributorTest(t, 3)
	initialSupply := s.TotalSupply()

	fb := FeeBreakdown{
		TotalFee:        300,
		ValidatorReward: 210, // 70%
		TreasuryAmount:  60,  // 20%
		BurnAmount:      30,  // 10%
	}

	payouts, err := d.DistributeFees(fb, validators)
	if err != nil {
		t.Fatal(err)
	}

	if len(payouts) != 3 {
		t.Fatalf("expected 3 payouts, got %d", len(payouts))
	}

	// Each validator should get 70 (210/3)
	totalPaid := uint64(0)
	for _, p := range payouts {
		totalPaid += p.Amount
	}
	if totalPaid != 210 {
		t.Errorf("total paid = %d, want 210", totalPaid)
	}

	// Treasury should have 60
	treasuryBal := s.GetBalance(types.HexToAddress("0xTREASURY000000000000000000000000000000"))
	if treasuryBal != 60 {
		t.Errorf("treasury balance = %d, want 60", treasuryBal)
	}

	// Total supply should increase by validator+treasury mints but decrease by burn
	expectedSupply := initialSupply + 210 + 60 - 30
	if s.TotalSupply() != expectedSupply {
		t.Errorf("total supply = %d, want %d", s.TotalSupply(), expectedSupply)
	}

	// Total burned should be 30
	if s.TotalBurned() != 30 {
		t.Errorf("total burned = %d, want 30", s.TotalBurned())
	}
}

func TestDistributor_ProportionalByStake(t *testing.T) {
	treasury := types.HexToAddress("0xTREASURY000000000000000000000000000000")
	genesis := &types.GenesisConfig{
		ChainID:      "test",
		AdminAddress: types.HexToAddress("0x01"),
		MaxSupply:    1_000_000_000,
		MinimumStake: 100,
	}
	s := state.NewStateDB(genesis)

	// Validator A: 30000 stake, Validator B: 10000 stake (3:1 ratio)
	addrA := types.HexToAddress("0xAA")
	addrB := types.HexToAddress("0xBB")
	s.AddBalance(addrA, 30000)
	s.AddBalance(addrB, 10000)
	s.Stake(addrA, 30000, 1000)
	s.Stake(addrB, 10000, 1000)

	d := NewDistributor(s, treasury)
	fb := FeeBreakdown{
		TotalFee:        400,
		ValidatorReward: 400,
		TreasuryAmount:  0,
		BurnAmount:      0,
	}

	payouts, err := d.DistributeFees(fb, []types.Address{addrA, addrB})
	if err != nil {
		t.Fatal(err)
	}

	// A should get 300 (3/4), B should get 100 (1/4)
	payoutMap := make(map[types.Address]uint64)
	for _, p := range payouts {
		payoutMap[p.Validator] = p.Amount
	}

	total := payoutMap[addrA] + payoutMap[addrB]
	if total != 400 {
		t.Errorf("total payouts = %d, want 400", total)
	}
	if payoutMap[addrA] <= payoutMap[addrB] {
		t.Errorf("validator A (30k stake) should get more than B (10k stake): A=%d, B=%d", payoutMap[addrA], payoutMap[addrB])
	}
}

func TestDistributor_SingleVoter(t *testing.T) {
	s, d, validators := setupDistributorTest(t, 3)

	fb := FeeBreakdown{
		TotalFee:        100,
		ValidatorReward: 70,
		TreasuryAmount:  20,
		BurnAmount:      10,
	}

	payouts, err := d.DistributeFees(fb, []types.Address{validators[0]})
	if err != nil {
		t.Fatal(err)
	}

	if len(payouts) != 1 {
		t.Fatalf("expected 1 payout, got %d", len(payouts))
	}
	if payouts[0].Amount != 70 {
		t.Errorf("single voter should get all 70, got %d", payouts[0].Amount)
	}

	treasuryBal := s.GetBalance(types.HexToAddress("0xTREASURY000000000000000000000000000000"))
	if treasuryBal != 20 {
		t.Errorf("treasury = %d, want 20", treasuryBal)
	}
}

func TestDistributor_ZeroFee(t *testing.T) {
	_, d, validators := setupDistributorTest(t, 3)

	payouts, err := d.DistributeFees(FeeBreakdown{}, validators)
	if err != nil {
		t.Fatal(err)
	}
	if payouts != nil {
		t.Error("expected nil payouts for zero fee")
	}
}

func TestDistributor_NoActiveVoters(t *testing.T) {
	treasury := types.HexToAddress("0xTREASURY000000000000000000000000000000")
	genesis := &types.GenesisConfig{
		ChainID:      "test",
		AdminAddress: types.HexToAddress("0x01"),
		MaxSupply:    1_000_000_000,
		MinimumStake: 100,
	}
	s := state.NewStateDB(genesis)
	d := NewDistributor(s, treasury)

	fb := FeeBreakdown{TotalFee: 100, ValidatorReward: 70}
	_, err := d.DistributeFees(fb, []types.Address{types.HexToAddress("0xNOBODY")})
	if err == nil {
		t.Error("expected error when no active voters")
	}
}
