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
	EvidenceDoubleSign = "double_sign"
	EvidenceEquivocate = "equivocation"
	EvidenceLiveness   = "liveness"
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
// EvidenceID identifies the underlying OFFENSE (not the submitted bytes): the same
// double-sign reported twice — even re-encoded — yields the same ID, so the executor
// can reject replays.
type SlashAction struct {
	Validator  types.Address `json:"validator"`
	Amount     uint64        `json:"amount"`
	Reason     string        `json:"reason"`
	EvidenceID types.Hash    `json:"evidenceId"`
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
		// Liveness evidence carries no cryptographic proof — accepting it would let
		// anyone slash any validator by claiming missed selections. Rejected until a
		// verifiable liveness protocol exists.
		return nil, fmt.Errorf("liveness evidence is not accepted: claims are not verifiable")
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
		Validator:  validator,
		Amount:     slashAmount,
		Reason:     fmt.Sprintf("double-sign at height %d", ev.Height),
		EvidenceID: doubleSignEvidenceID(validator, ev.Height),
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
		Validator:  validator,
		Amount:     slashAmount,
		Reason:     fmt.Sprintf("VRF equivocation on seed %s", ev.Seed.Hex()),
		EvidenceID: equivocationEvidenceID(validator, ev.Seed),
	}, nil
}

// doubleSignEvidenceID derives the canonical offense identity for a double-sign:
// one slash per validator per height, regardless of how the evidence is encoded.
func doubleSignEvidenceID(validator types.Address, height uint64) types.Hash {
	buf := make([]byte, 0, 2+20+8)
	buf = append(buf, "ds"...)
	buf = append(buf, validator[:]...)
	var h [8]byte
	for i := 0; i < 8; i++ {
		h[i] = byte(height >> (56 - 8*i))
	}
	buf = append(buf, h[:]...)
	return pcrypto.SHA256Hash(buf)
}

// equivocationEvidenceID derives the canonical offense identity for VRF
// equivocation: one slash per validator per seed.
func equivocationEvidenceID(validator types.Address, seed types.Hash) types.Hash {
	buf := make([]byte, 0, 2+20+32)
	buf = append(buf, "eq"...)
	buf = append(buf, validator[:]...)
	buf = append(buf, seed[:]...)
	return pcrypto.SHA256Hash(buf)
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
