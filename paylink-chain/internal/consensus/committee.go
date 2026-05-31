package consensus

import (
	"crypto/ed25519"
	"math/big"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// CommitteeMember represents a validator selected for a proof committee.
type CommitteeMember struct {
	Address      types.Address
	VRFOutput    types.Hash
	VRFProof     []byte
	StakeWeight  uint64
	VRFPublicKey ed25519.PublicKey
}

// CommitteeSelector selects validator committees using VRF and stake weighting.
type CommitteeSelector struct {
	state           *state.StateDB
	targetSize      int     // target committee size (e.g., 5)
	quorumFraction  float64 // fraction needed for quorum (e.g., 0.6)
}

// NewCommitteeSelector creates a new committee selector.
func NewCommitteeSelector(s *state.StateDB, targetSize int, quorumFraction float64) *CommitteeSelector {
	if targetSize <= 0 {
		targetSize = 5
	}
	if quorumFraction <= 0 || quorumFraction > 1 {
		quorumFraction = 0.6
	}
	return &CommitteeSelector{
		state:          s,
		targetSize:     targetSize,
		quorumFraction: quorumFraction,
	}
}

// SelectCommittee determines which validators are eligible for a given seed.
// Each validator with a registered VRF key evaluates their VRF against the seed.
// A validator is eligible if: vrfOutput < threshold * (validatorStake / totalStake).
//
// The seed should be deterministic and unpredictable, e.g., SHA256(lastBlockHash || payLinkID).
func (cs *CommitteeSelector) SelectCommittee(seed types.Hash, vrfKeys map[types.Address]*pcrypto.ECVRF) []CommitteeMember {
	validators := cs.state.GetActiveValidatorsWithStake()
	if len(validators) == 0 {
		return nil
	}

	totalStake := uint64(0)
	for _, v := range validators {
		totalStake += v.StakedAmount
	}
	if totalStake == 0 {
		return nil
	}

	var committee []CommitteeMember
	for _, v := range validators {
		vrf, ok := vrfKeys[v.Address]
		if !ok || vrf == nil {
			continue // no VRF key registered
		}

		output, proof, err := vrf.Evaluate(seed[:])
		if err != nil {
			continue
		}

		if isEligible(output, v.StakedAmount, totalStake, len(validators), cs.targetSize) {
			committee = append(committee, CommitteeMember{
				Address:      v.Address,
				VRFOutput:    output,
				VRFProof:     proof,
				StakeWeight:  v.StakedAmount,
				VRFPublicKey: vrf.PublicKey(),
			})
		}
	}

	return committee
}

// VerifyCommitteeMembership verifies that a validator is a legitimate committee member
// by checking their VRF proof and eligibility.
func (cs *CommitteeSelector) VerifyCommitteeMembership(
	seed types.Hash,
	member CommitteeMember,
	totalStake uint64,
	numValidators int,
) bool {
	// Verify VRF proof
	if !pcrypto.VerifyVRFProof(seed[:], member.VRFOutput, member.VRFProof) {
		return false
	}

	// Check eligibility threshold
	return isEligible(member.VRFOutput, member.StakeWeight, totalStake, numValidators, cs.targetSize)
}

// RequiredQuorum returns the minimum number of votes needed given a committee size.
func (cs *CommitteeSelector) RequiredQuorum(committeeSize int) int {
	q := int(float64(committeeSize)*cs.quorumFraction + 0.999) // ceil
	if q < 1 {
		q = 1
	}
	return q
}

// ComputeSeed computes the deterministic seed for committee selection.
// seed = SHA256(lastBlockHash || payLinkID)
func ComputeSeed(lastBlockHash, payLinkID types.Hash) types.Hash {
	return pcrypto.CombineHashes(lastBlockHash, payLinkID)
}

// isEligible checks if a VRF output falls below the stake-weighted threshold.
// Uses Algorand-style sortition: each validator is independently selected with
// probability p_i = targetSize * validatorStake / totalStake.
//
// threshold = MaxUint256 * targetSize * validatorStake / totalStake
// (capped at MaxUint256 so probability never exceeds 1)
func isEligible(vrfOutput types.Hash, validatorStake, totalStake uint64, numValidators, targetSize int) bool {
	if totalStake == 0 {
		return false
	}

	// Convert VRF output to big.Int
	outputInt := new(big.Int).SetBytes(vrfOutput[:])

	// MaxUint256
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	// threshold = maxUint256 * targetSize * validatorStake / totalStake
	numerator := new(big.Int).Mul(maxUint256, big.NewInt(int64(targetSize)))
	numerator.Mul(numerator, new(big.Int).SetUint64(validatorStake))

	denominator := new(big.Int).SetUint64(totalStake)

	threshold := new(big.Int).Div(numerator, denominator)

	// Cap at maxUint256 (probability can't exceed 1)
	if threshold.Cmp(maxUint256) > 0 {
		threshold.Set(maxUint256)
	}

	return outputInt.Cmp(threshold) < 0
}
