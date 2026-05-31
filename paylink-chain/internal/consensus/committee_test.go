package consensus

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// setupValidators creates a StateDB with N active validators at the given stakes,
// registers VRF keys, and returns the VRF instances keyed by address.
func setupValidators(t *testing.T, stakes []uint64) (*state.StateDB, map[types.Address]*pcrypto.ECVRF) {
	t.Helper()

	genesis := &types.GenesisConfig{
		ChainID:             "test",
		AdminAddress:        types.HexToAddress("0x01"),
		MaxSupply:           1_000_000_000,
		MinimumStake:        100,
		WithdrawalCooldown:  86400,
		RequiredValidations: 3,
		TargetCommitteeSize: 5,
		QuorumFraction:      0.6,
	}

	s := state.NewStateDB(genesis)
	vrfKeys := make(map[types.Address]*pcrypto.ECVRF)

	for i, stake := range stakes {
		addr := types.HexToAddress(fmt.Sprintf("0x%040x", i+100))

		// Fund and stake
		s.AddBalance(addr, stake)
		if err := s.Stake(addr, stake, 1000); err != nil {
			t.Fatalf("stake validator %d: %v", i, err)
		}

		// Generate and register VRF key
		pub, priv, err := pcrypto.GenerateVRFKey()
		if err != nil {
			t.Fatalf("generate VRF key %d: %v", i, err)
		}
		vrf, err := pcrypto.NewECVRF(priv)
		if err != nil {
			t.Fatalf("new ECVRF %d: %v", i, err)
		}
		if err := s.SetVRFPublicKey(addr, pub); err != nil {
			t.Fatalf("set VRF key %d: %v", i, err)
		}

		vrfKeys[addr] = vrf
	}

	return s, vrfKeys
}

func TestCommitteeSelector_EqualStake(t *testing.T) {
	stakes := []uint64{10000, 10000, 10000, 10000, 10000}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 5, 0.6)

	// Run selection many times and check that all validators get selected roughly equally
	selectionCount := make(map[types.Address]int)
	rounds := 500
	for i := 0; i < rounds; i++ {
		seed := pcrypto.SHA256Hash([]byte(fmt.Sprintf("seed-%d", i)))
		committee := cs.SelectCommittee(seed, vrfKeys)
		for _, m := range committee {
			selectionCount[m.Address]++
		}
	}

	// With 5 validators and target 5, each should be selected ~500 times
	// Allow wide tolerance for randomness
	for addr, count := range selectionCount {
		if count < 100 || count > rounds {
			t.Errorf("validator %s selected %d times out of %d rounds (expected ~%d)", addr, count, rounds, rounds)
		}
	}
}

func TestCommitteeSelector_HigherStakeMoreFrequent(t *testing.T) {
	// One validator has 10x the stake of others
	stakes := []uint64{100000, 10000, 10000, 10000, 10000}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 3, 0.6)

	selectionCount := make(map[types.Address]int)
	rounds := 1000
	for i := 0; i < rounds; i++ {
		seed := pcrypto.SHA256Hash([]byte(fmt.Sprintf("seed-%d", i)))
		committee := cs.SelectCommittee(seed, vrfKeys)
		for _, m := range committee {
			selectionCount[m.Address]++
		}
	}

	// Get the addresses
	validators := s.GetActiveValidatorsWithStake()
	highStakeAddr := validators[0].Address
	lowStakeAddr := validators[1].Address

	highCount := selectionCount[highStakeAddr]
	lowCount := selectionCount[lowStakeAddr]

	// High-stake validator should be selected more often
	if highCount <= lowCount {
		t.Errorf("high-stake validator (%d selections) should be selected more than low-stake (%d)", highCount, lowCount)
	}
}

func TestCommitteeSelector_Determinism(t *testing.T) {
	stakes := []uint64{10000, 10000, 10000}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 3, 0.6)
	seed := pcrypto.SHA256Hash([]byte("deterministic-seed"))

	committee1 := cs.SelectCommittee(seed, vrfKeys)
	committee2 := cs.SelectCommittee(seed, vrfKeys)

	if len(committee1) != len(committee2) {
		t.Fatalf("committee sizes differ: %d vs %d", len(committee1), len(committee2))
	}

	for i := range committee1 {
		if committee1[i].Address != committee2[i].Address {
			t.Errorf("member %d differs: %s vs %s", i, committee1[i].Address, committee2[i].Address)
		}
		if committee1[i].VRFOutput != committee2[i].VRFOutput {
			t.Errorf("VRF output %d differs", i)
		}
	}
}

func TestCommitteeSelector_VRFProofsVerifiable(t *testing.T) {
	stakes := []uint64{10000, 10000, 10000, 10000, 10000}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 5, 0.6)
	seed := pcrypto.SHA256Hash([]byte("verify-proofs"))

	committee := cs.SelectCommittee(seed, vrfKeys)

	totalStake := s.TotalStaked()
	numValidators := s.ActiveValidatorCount()

	for _, m := range committee {
		if !cs.VerifyCommitteeMembership(seed, m, totalStake, numValidators) {
			t.Errorf("failed to verify committee membership for %s", m.Address)
		}
	}
}

func TestCommitteeSelector_NoValidators(t *testing.T) {
	genesis := &types.GenesisConfig{
		ChainID:      "test",
		AdminAddress: types.HexToAddress("0x01"),
		MaxSupply:    1_000_000_000,
		MinimumStake: 100,
	}
	s := state.NewStateDB(genesis)
	cs := NewCommitteeSelector(s, 5, 0.6)

	seed := pcrypto.SHA256Hash([]byte("no-validators"))
	committee := cs.SelectCommittee(seed, nil)

	if len(committee) != 0 {
		t.Errorf("expected empty committee, got %d members", len(committee))
	}
}

func TestCommitteeSelector_NoVRFKeys(t *testing.T) {
	stakes := []uint64{10000, 10000, 10000}
	s, _ := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 3, 0.6)
	seed := pcrypto.SHA256Hash([]byte("no-vrf-keys"))

	// Pass empty VRF keys map
	committee := cs.SelectCommittee(seed, make(map[types.Address]*pcrypto.ECVRF))
	if len(committee) != 0 {
		t.Errorf("expected empty committee without VRF keys, got %d", len(committee))
	}
}

func TestCommitteeSelector_VerifyRejectsWrongSeed(t *testing.T) {
	stakes := []uint64{10000, 10000, 10000}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 5, 0.6)
	seed := pcrypto.SHA256Hash([]byte("correct-seed"))

	committee := cs.SelectCommittee(seed, vrfKeys)
	if len(committee) == 0 {
		t.Skip("no committee members selected")
	}

	wrongSeed := pcrypto.SHA256Hash([]byte("wrong-seed"))
	totalStake := s.TotalStaked()
	numValidators := s.ActiveValidatorCount()

	for _, m := range committee {
		if cs.VerifyCommitteeMembership(wrongSeed, m, totalStake, numValidators) {
			t.Error("should reject committee membership with wrong seed")
		}
	}
}

func TestCommitteeSelector_VerifyRejectsFakeProof(t *testing.T) {
	stakes := []uint64{10000, 10000, 10000}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, 5, 0.6)
	seed := pcrypto.SHA256Hash([]byte("fake-proof"))

	committee := cs.SelectCommittee(seed, vrfKeys)
	if len(committee) == 0 {
		t.Skip("no committee members selected")
	}

	totalStake := s.TotalStaked()
	numValidators := s.ActiveValidatorCount()

	// Tamper with the first member's proof
	fake := committee[0]
	fake.VRFProof = make([]byte, len(committee[0].VRFProof))
	copy(fake.VRFProof, committee[0].VRFProof)
	fake.VRFProof[ed25519.PublicKeySize+5] ^= 0xFF

	if cs.VerifyCommitteeMembership(seed, fake, totalStake, numValidators) {
		t.Error("should reject fake proof")
	}
}

func TestComputeSeed(t *testing.T) {
	blockHash := pcrypto.SHA256Hash([]byte("block-1"))
	plId := pcrypto.SHA256Hash([]byte("paylink-1"))

	seed1 := ComputeSeed(blockHash, plId)
	seed2 := ComputeSeed(blockHash, plId)

	if seed1 != seed2 {
		t.Error("ComputeSeed not deterministic")
	}

	// Different inputs should produce different seeds
	plId2 := pcrypto.SHA256Hash([]byte("paylink-2"))
	seed3 := ComputeSeed(blockHash, plId2)
	if seed1 == seed3 {
		t.Error("different inputs should produce different seeds")
	}
}

func TestRequiredQuorum(t *testing.T) {
	cs := NewCommitteeSelector(nil, 5, 0.6)

	tests := []struct {
		committeeSize int
		expected      int
	}{
		{5, 3},  // ceil(5 * 0.6) = 3
		{3, 2},  // ceil(3 * 0.6) = 2
		{1, 1},  // ceil(1 * 0.6) = 1
		{10, 6}, // ceil(10 * 0.6) = 6
	}

	for _, tt := range tests {
		got := cs.RequiredQuorum(tt.committeeSize)
		if got != tt.expected {
			t.Errorf("RequiredQuorum(%d) = %d, want %d", tt.committeeSize, got, tt.expected)
		}
	}
}
