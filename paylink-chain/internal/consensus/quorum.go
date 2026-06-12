package consensus

import (
	"bytes"
	"fmt"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/types"
)

// Vote represents a validator's vote on a PayLink proof.
type Vote struct {
	Validator types.Address
	ProofHash types.Hash
	VRFOutput types.Hash
	VRFProof  []byte
}

// QuorumChecker verifies that enough committee members have voted.
type QuorumChecker struct {
	quorumFraction float64
}

// NewQuorumChecker creates a new quorum checker.
func NewQuorumChecker(quorumFraction float64) *QuorumChecker {
	if quorumFraction <= 0 || quorumFraction > 1 {
		quorumFraction = 0.6
	}
	return &QuorumChecker{quorumFraction: quorumFraction}
}

// CheckQuorum verifies that enough valid committee members have voted on a proof.
// It validates each vote's VRF proof and checks that the required quorum is met.
func (qc *QuorumChecker) CheckQuorum(
	seed types.Hash,
	committee []CommitteeMember,
	votes []Vote,
) (bool, error) {
	if len(committee) == 0 {
		return false, fmt.Errorf("empty committee")
	}

	requiredVotes := qc.RequiredVotes(len(committee))

	// Build committee member lookup
	memberSet := make(map[types.Address]CommitteeMember)
	for _, m := range committee {
		memberSet[m.Address] = m
	}

	// Validate votes
	validVotes := 0
	seen := make(map[types.Address]bool)
	var consensusProof types.Hash

	for _, vote := range votes {
		// Skip duplicate votes
		if seen[vote.Validator] {
			continue
		}

		// Check voter is in committee
		member, ok := memberSet[vote.Validator]
		if !ok {
			continue // not a committee member
		}

		// Bind the proof to the validator's REGISTERED VRF key. VerifyVRFProof
		// reads the pubkey out of the proof itself, so without this check anyone
		// could mint a "valid" proof with a fresh key.
		votePub, err := pcrypto.VRFProofPublicKey(vote.VRFProof)
		if err != nil || len(member.VRFPublicKey) == 0 || !bytes.Equal(votePub, member.VRFPublicKey) {
			continue // proof not bound to the member's registered key
		}

		// Verify VRF proof of the voter's committee membership
		if !pcrypto.VerifyVRFProof(seed[:], vote.VRFOutput, vote.VRFProof) {
			continue // invalid VRF proof
		}

		// Verify VRF output matches the committee member's output
		if vote.VRFOutput != member.VRFOutput {
			continue // VRF output mismatch
		}

		// All voters must agree on the same proof hash
		if validVotes == 0 {
			consensusProof = vote.ProofHash
		} else if vote.ProofHash != consensusProof {
			continue // conflicting proof
		}

		seen[vote.Validator] = true
		validVotes++
	}

	return validVotes >= requiredVotes, nil
}

// RequiredVotes returns the minimum number of votes needed for a given committee size.
func (qc *QuorumChecker) RequiredVotes(committeeSize int) int {
	q := int(float64(committeeSize)*qc.quorumFraction + 0.999) // ceil
	if q < 1 {
		q = 1
	}
	return q
}
