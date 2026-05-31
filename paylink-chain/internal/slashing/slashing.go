package slashing

import (
	"encoding/json"
	"fmt"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// Evidence types for slashing offenses.
const (
	EvidenceDoubleSign   = "double_sign"
	EvidenceEquivocate   = "equivocation"
	EvidenceLiveness     = "liveness"
)

// Slash percentages (of current stake).
const (
	DoubleSignSlashPct   = 50 // 50% for signing two blocks at same height
	EquivocationSlashPct = 25 // 25% for two VRF proofs on same seed
	LivenessSlashPct     = 5  // 5% per missed consecutive committee selection
)

// DoubleSignEvidence contains proof of a validator signing two different blocks at the same height.
type DoubleSignEvidence struct {
	Height     uint64     `json:"height"`
	BlockHash1 types.Hash `json:"blockHash1"`
	BlockHash2 types.Hash `json:"blockHash2"`
	Signature1 []byte     `json:"signature1"`
	Signature2 []byte     `json:"signature2"`
	PublicKey  []byte     `json:"publicKey"` // 65-byte uncompressed ECDSA public key of the offender
}

// EquivocationEvidence contains proof of a validator producing two VRF outputs for the same seed.
type EquivocationEvidence struct {
	Seed      types.Hash `json:"seed"`
	VRFProof1 []byte     `json:"vrfProof1"`
	VRFProof2 []byte     `json:"vrfProof2"`
}

// LivenessEvidence tracks consecutive missed committee selections.
type LivenessEvidence struct {
	MissedCount uint64       `json:"missedCount"`
	StartHeight uint64       `json:"startHeight"`
	EndHeight   uint64       `json:"endHeight"`
	Seeds       []types.Hash `json:"seeds"` // committee seeds for verification
}

// SlashAction describes the resulting penalty from processing evidence.
type SlashAction struct {
	Validator types.Address `json:"validator"`
	Amount    uint64        `json:"amount"`
	Reason    string        `json:"reason"`
}

// SlashingDetector validates evidence and computes slash amounts.
type SlashingDetector struct {
	state *state.StateDB
}

// NewSlashingDetector creates a new slashing detector.
func NewSlashingDetector(s *state.StateDB) *SlashingDetector {
	return &SlashingDetector{state: s}
}

// ProcessEvidence validates the submitted evidence and returns the resulting slash action.
func (sd *SlashingDetector) ProcessEvidence(
	evidenceType string,
	validator types.Address,
	rawData json.RawMessage,
) (*SlashAction, error) {
	v := sd.state.GetValidator(validator)
	if v == nil || v.StakedAmount == 0 {
		return nil, fmt.Errorf("validator %s has no stake", validator)
	}

	switch evidenceType {
	case EvidenceDoubleSign:
		return sd.processDoubleSign(validator, v, rawData)
	case EvidenceEquivocate:
		return sd.processEquivocation(validator, v, rawData)
	case EvidenceLiveness:
		return sd.processLiveness(validator, v, rawData)
	default:
		return nil, fmt.Errorf("unknown evidence type: %s", evidenceType)
	}
}

func (sd *SlashingDetector) processDoubleSign(
	validator types.Address,
	v *types.ValidatorInfo,
	rawData json.RawMessage,
) (*SlashAction, error) {
	var ev DoubleSignEvidence
	if err := json.Unmarshal(rawData, &ev); err != nil {
		return nil, fmt.Errorf("invalid double-sign evidence: %w", err)
	}

	if ev.BlockHash1 == ev.BlockHash2 {
		return nil, fmt.Errorf("block hashes are identical")
	}

	// Parse the public key and verify it maps to the claimed validator
	pub, err := pcrypto.UnmarshalPublicKey(ev.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key in evidence: %w", err)
	}
	derivedAddr := pcrypto.PubkeyToAddress(pub)
	if derivedAddr != validator {
		return nil, fmt.Errorf("public key does not match validator %s (got %s)", validator, derivedAddr)
	}

	// Verify both signatures are valid for the respective block hashes
	if !pcrypto.Verify(ev.BlockHash1, ev.Signature1, pub) {
		return nil, fmt.Errorf("signature1 invalid for blockHash1")
	}
	if !pcrypto.Verify(ev.BlockHash2, ev.Signature2, pub) {
		return nil, fmt.Errorf("signature2 invalid for blockHash2")
	}

	slashAmount := v.StakedAmount * DoubleSignSlashPct / 100
	if slashAmount == 0 {
		slashAmount = v.StakedAmount
	}

	return &SlashAction{
		Validator: validator,
		Amount:    slashAmount,
		Reason:    fmt.Sprintf("double-sign at height %d", ev.Height),
	}, nil
}

func (sd *SlashingDetector) processEquivocation(
	validator types.Address,
	v *types.ValidatorInfo,
	rawData json.RawMessage,
) (*SlashAction, error) {
	var ev EquivocationEvidence
	if err := json.Unmarshal(rawData, &ev); err != nil {
		return nil, fmt.Errorf("invalid equivocation evidence: %w", err)
	}

	// Both proofs must be valid VRF proofs for the same seed
	if !pcrypto.VerifyVRFProof(ev.Seed[:], vrfOutputFromProof(ev.VRFProof1), ev.VRFProof1) {
		return nil, fmt.Errorf("VRF proof 1 invalid")
	}
	if !pcrypto.VerifyVRFProof(ev.Seed[:], vrfOutputFromProof(ev.VRFProof2), ev.VRFProof2) {
		return nil, fmt.Errorf("VRF proof 2 invalid")
	}

	// Extract public keys from proofs and verify both belong to the same validator
	pub1, err := pcrypto.VRFProofPublicKey(ev.VRFProof1)
	if err != nil {
		return nil, fmt.Errorf("cannot extract pubkey from proof1: %w", err)
	}
	pub2, err := pcrypto.VRFProofPublicKey(ev.VRFProof2)
	if err != nil {
		return nil, fmt.Errorf("cannot extract pubkey from proof2: %w", err)
	}

	// Verify the VRF key belongs to this validator
	storedKey := sd.state.GetVRFPublicKey(validator)
	if storedKey == nil {
		return nil, fmt.Errorf("validator %s has no VRF public key", validator)
	}
	if !bytesEqual(pub1, storedKey) || !bytesEqual(pub2, storedKey) {
		return nil, fmt.Errorf("VRF proof public key does not match validator %s", validator)
	}

	// The proofs must be different bytes (equivocation = two different proofs for same input)
	if bytesEqual(ev.VRFProof1, ev.VRFProof2) {
		return nil, fmt.Errorf("proofs are identical -- not equivocation")
	}

	slashAmount := v.StakedAmount * EquivocationSlashPct / 100
	if slashAmount == 0 {
		slashAmount = v.StakedAmount
	}

	return &SlashAction{
		Validator: validator,
		Amount:    slashAmount,
		Reason:    fmt.Sprintf("VRF equivocation on seed %s", ev.Seed.Hex()),
	}, nil
}

func (sd *SlashingDetector) processLiveness(
	validator types.Address,
	v *types.ValidatorInfo,
	rawData json.RawMessage,
) (*SlashAction, error) {
	var ev LivenessEvidence
	if err := json.Unmarshal(rawData, &ev); err != nil {
		return nil, fmt.Errorf("invalid liveness evidence: %w", err)
	}

	if ev.MissedCount == 0 {
		return nil, fmt.Errorf("missed count must be > 0")
	}

	totalPct := ev.MissedCount * LivenessSlashPct
	if totalPct > 100 {
		totalPct = 100
	}

	slashAmount := v.StakedAmount * totalPct / 100
	if slashAmount == 0 {
		slashAmount = 1
	}

	return &SlashAction{
		Validator: validator,
		Amount:    slashAmount,
		Reason:    fmt.Sprintf("liveness failure: %d missed from height %d to %d", ev.MissedCount, ev.StartHeight, ev.EndHeight),
	}, nil
}

// vrfOutputFromProof re-derives the VRF output from a proof.
// Proof format: pubkey(32) || signature(64). Output = SHA256(signature).
func vrfOutputFromProof(proof []byte) types.Hash {
	if len(proof) < 96 {
		return types.ZeroHash
	}
	return pcrypto.SHA256Hash(proof[32:96])
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
