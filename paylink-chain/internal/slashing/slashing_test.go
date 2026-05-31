package slashing

import (
	"crypto/ecdsa"
	"encoding/json"
	"testing"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// helper to create a stateDB with a staked validator.
func setupSlashingState(t *testing.T, addr types.Address, stakeAmount uint64) *state.StateDB {
	t.Helper()
	genesis := &types.GenesisConfig{
		ChainID:              "test",
		InitialSupply:        1_000_000,
		MinimumStake:         1_000,
		WithdrawalCooldown:   100,
		RequiredValidations:  3,
		TargetCommitteeSize:  5,
		QuorumFraction:       0.6,
		FeeRateBasisPoints:   50,
		ValidatorRewardShare: 70,
		TreasuryShare:        20,
		BurnShare:            10,
		InitialBalances: []types.GenesisBalance{
			{Address: addr, Balance: stakeAmount * 2},
		},
	}
	s := state.NewStateDB(genesis)
	// Stake the validator
	if err := s.SubBalance(addr, stakeAmount); err != nil {
		t.Fatalf("sub balance: %v", err)
	}
	if err := s.Stake(addr, stakeAmount, 1); err != nil {
		t.Fatalf("stake: %v", err)
	}
	return s
}

func signBlock(t *testing.T, key *ecdsa.PrivateKey, blockHash types.Hash) []byte {
	t.Helper()
	sig, err := pcrypto.Sign(blockHash, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return sig
}

func TestSlashing_DoubleSign(t *testing.T) {
	key, err := pcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	addr := pcrypto.PrivateKeyToAddress(key)
	s := setupSlashingState(t, addr, 100_000)

	hash1 := pcrypto.SHA256Hash([]byte("block-A"))
	hash2 := pcrypto.SHA256Hash([]byte("block-B"))
	sig1 := signBlock(t, key, hash1)
	sig2 := signBlock(t, key, hash2)

	ev := DoubleSignEvidence{
		Height:     42,
		BlockHash1: hash1,
		BlockHash2: hash2,
		Signature1: sig1,
		Signature2: sig2,
		PublicKey:  pcrypto.MarshalPublicKey(&key.PublicKey),
	}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	action, err := detector.ProcessEvidence(EvidenceDoubleSign, addr, rawData)
	if err != nil {
		t.Fatalf("ProcessEvidence: %v", err)
	}

	if action.Validator != addr {
		t.Errorf("validator = %s, want %s", action.Validator, addr)
	}
	// 50% of 100,000 = 50,000
	if action.Amount != 50_000 {
		t.Errorf("amount = %d, want 50000", action.Amount)
	}
}

func TestSlashing_DoubleSign_IdenticalHashesRejected(t *testing.T) {
	key, _ := pcrypto.GenerateKey()
	addr := pcrypto.PrivateKeyToAddress(key)
	s := setupSlashingState(t, addr, 100_000)

	hash := pcrypto.SHA256Hash([]byte("block-A"))
	sig := signBlock(t, key, hash)

	ev := DoubleSignEvidence{
		Height:     42,
		BlockHash1: hash,
		BlockHash2: hash, // same!
		Signature1: sig,
		Signature2: sig,
		PublicKey:  pcrypto.MarshalPublicKey(&key.PublicKey),
	}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	_, err := detector.ProcessEvidence(EvidenceDoubleSign, addr, rawData)
	if err == nil {
		t.Fatal("expected error for identical block hashes")
	}
}

func TestSlashing_DoubleSign_WrongKey(t *testing.T) {
	key1, _ := pcrypto.GenerateKey()
	key2, _ := pcrypto.GenerateKey()
	addr1 := pcrypto.PrivateKeyToAddress(key1)
	s := setupSlashingState(t, addr1, 100_000)

	hash1 := pcrypto.SHA256Hash([]byte("block-A"))
	hash2 := pcrypto.SHA256Hash([]byte("block-B"))

	ev := DoubleSignEvidence{
		Height:     42,
		BlockHash1: hash1,
		BlockHash2: hash2,
		Signature1: signBlock(t, key2, hash1), // signed by wrong key
		Signature2: signBlock(t, key2, hash2),
		PublicKey:  pcrypto.MarshalPublicKey(&key2.PublicKey), // key2, not addr1
	}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	_, err := detector.ProcessEvidence(EvidenceDoubleSign, addr1, rawData)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestSlashing_Liveness(t *testing.T) {
	addr := types.Address{0x01}
	s := setupSlashingState(t, addr, 100_000)

	ev := LivenessEvidence{
		MissedCount: 3,
		StartHeight: 100,
		EndHeight:   102,
	}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	action, err := detector.ProcessEvidence(EvidenceLiveness, addr, rawData)
	if err != nil {
		t.Fatalf("ProcessEvidence: %v", err)
	}

	// 3 * 5% = 15% of 100,000 = 15,000
	if action.Amount != 15_000 {
		t.Errorf("amount = %d, want 15000", action.Amount)
	}
}

func TestSlashing_Liveness_CappedAt100Pct(t *testing.T) {
	addr := types.Address{0x02}
	s := setupSlashingState(t, addr, 100_000)

	ev := LivenessEvidence{
		MissedCount: 50, // 50 * 5% = 250%, capped at 100%
		StartHeight: 100,
		EndHeight:   149,
	}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	action, err := detector.ProcessEvidence(EvidenceLiveness, addr, rawData)
	if err != nil {
		t.Fatalf("ProcessEvidence: %v", err)
	}

	// Capped at 100% = 100,000
	if action.Amount != 100_000 {
		t.Errorf("amount = %d, want 100000", action.Amount)
	}
}

func TestSlashing_Liveness_ZeroMissed(t *testing.T) {
	addr := types.Address{0x03}
	s := setupSlashingState(t, addr, 100_000)

	ev := LivenessEvidence{MissedCount: 0}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	_, err := detector.ProcessEvidence(EvidenceLiveness, addr, rawData)
	if err == nil {
		t.Fatal("expected error for 0 missed")
	}
}

func TestSlashing_NoStake(t *testing.T) {
	addr := types.Address{0x99}
	genesis := &types.GenesisConfig{
		ChainID:       "test",
		InitialSupply: 1_000_000,
		MinimumStake:  1_000,
	}
	s := state.NewStateDB(genesis)

	ev := LivenessEvidence{MissedCount: 1}
	rawData, _ := json.Marshal(ev)

	detector := NewSlashingDetector(s)
	_, err := detector.ProcessEvidence(EvidenceLiveness, addr, rawData)
	if err == nil {
		t.Fatal("expected error for unstaked validator")
	}
}

func TestSlashing_UnknownType(t *testing.T) {
	addr := types.Address{0x01}
	s := setupSlashingState(t, addr, 100_000)

	detector := NewSlashingDetector(s)
	_, err := detector.ProcessEvidence("unknown_type", addr, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown evidence type")
	}
}
