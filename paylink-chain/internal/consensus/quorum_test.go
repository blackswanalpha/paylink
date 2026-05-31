package consensus

import (
	"fmt"
	"testing"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/types"
)

func buildCommitteeAndVotes(t *testing.T, numValidators, committeeTarget int) (types.Hash, []CommitteeMember, map[types.Address]*pcrypto.ECVRF) {
	t.Helper()

	stakes := make([]uint64, numValidators)
	for i := range stakes {
		stakes[i] = 10000
	}
	s, vrfKeys := setupValidators(t, stakes)

	cs := NewCommitteeSelector(s, committeeTarget, 0.6)

	// Try multiple seeds to find one that produces a committee of at least 3
	for i := 0; i < 1000; i++ {
		seed := pcrypto.SHA256Hash([]byte(fmt.Sprintf("quorum-seed-%d", i)))
		committee := cs.SelectCommittee(seed, vrfKeys)
		if len(committee) >= 3 {
			return seed, committee, vrfKeys
		}
	}

	t.Fatal("could not find seed producing committee of 3+")
	return types.Hash{}, nil, nil
}

func TestQuorumChecker_ExactQuorum(t *testing.T) {
	seed, committee, vrfKeys := buildCommitteeAndVotes(t, 5, 5)
	_ = vrfKeys

	qc := NewQuorumChecker(0.6)
	required := qc.RequiredVotes(len(committee))

	// Create exactly required votes
	var votes []Vote
	for i := 0; i < required && i < len(committee); i++ {
		votes = append(votes, Vote{
			Validator: committee[i].Address,
			ProofHash: pcrypto.SHA256Hash([]byte("proof-data")),
			VRFOutput: committee[i].VRFOutput,
			VRFProof:  committee[i].VRFProof,
		})
	}

	ok, err := qc.CheckQuorum(seed, committee, votes)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("quorum should pass with %d of %d votes (required: %d)", len(votes), len(committee), required)
	}
}

func TestQuorumChecker_InsufficientVotes(t *testing.T) {
	seed, committee, _ := buildCommitteeAndVotes(t, 5, 5)

	qc := NewQuorumChecker(0.6)
	required := qc.RequiredVotes(len(committee))

	// Create fewer than required votes
	numVotes := required - 1
	if numVotes < 0 {
		numVotes = 0
	}

	var votes []Vote
	for i := 0; i < numVotes && i < len(committee); i++ {
		votes = append(votes, Vote{
			Validator: committee[i].Address,
			ProofHash: pcrypto.SHA256Hash([]byte("proof-data")),
			VRFOutput: committee[i].VRFOutput,
			VRFProof:  committee[i].VRFProof,
		})
	}

	ok, err := qc.CheckQuorum(seed, committee, votes)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("quorum should fail with %d of %d votes (required: %d)", numVotes, len(committee), required)
	}
}

func TestQuorumChecker_AllVotes(t *testing.T) {
	seed, committee, _ := buildCommitteeAndVotes(t, 5, 5)

	qc := NewQuorumChecker(0.6)

	proofHash := pcrypto.SHA256Hash([]byte("proof-data"))
	var votes []Vote
	for _, m := range committee {
		votes = append(votes, Vote{
			Validator: m.Address,
			ProofHash: proofHash,
			VRFOutput: m.VRFOutput,
			VRFProof:  m.VRFProof,
		})
	}

	ok, err := qc.CheckQuorum(seed, committee, votes)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("quorum should pass with all votes")
	}
}

func TestQuorumChecker_DuplicateVotesIgnored(t *testing.T) {
	seed, committee, _ := buildCommitteeAndVotes(t, 5, 5)

	qc := NewQuorumChecker(0.6)
	required := qc.RequiredVotes(len(committee))

	// Submit only 1 unique vote, duplicated many times
	if len(committee) == 0 {
		t.Skip("no committee members")
	}

	var votes []Vote
	for i := 0; i < required+5; i++ {
		votes = append(votes, Vote{
			Validator: committee[0].Address,
			ProofHash: pcrypto.SHA256Hash([]byte("proof-data")),
			VRFOutput: committee[0].VRFOutput,
			VRFProof:  committee[0].VRFProof,
		})
	}

	ok, err := qc.CheckQuorum(seed, committee, votes)
	if err != nil {
		t.Fatal(err)
	}
	if ok && required > 1 {
		t.Error("quorum should fail with duplicate votes from single validator")
	}
}

func TestQuorumChecker_NonMemberVotesIgnored(t *testing.T) {
	seed, committee, _ := buildCommitteeAndVotes(t, 5, 5)

	qc := NewQuorumChecker(0.6)

	// Create votes from fake addresses not in committee
	var votes []Vote
	for i := 0; i < 10; i++ {
		fakeAddr := types.HexToAddress(fmt.Sprintf("0x%040x", i+9000))
		votes = append(votes, Vote{
			Validator: fakeAddr,
			ProofHash: pcrypto.SHA256Hash([]byte("proof")),
			VRFOutput: types.Hash{},
			VRFProof:  make([]byte, 96),
		})
	}

	ok, err := qc.CheckQuorum(seed, committee, votes)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("quorum should fail with only non-member votes")
	}
}

func TestQuorumChecker_ConflictingProofsRejected(t *testing.T) {
	seed, committee, _ := buildCommitteeAndVotes(t, 5, 5)

	qc := NewQuorumChecker(0.6)

	if len(committee) < 3 {
		t.Skip("need at least 3 committee members")
	}

	// First vote with proof-A, rest with proof-B
	var votes []Vote
	votes = append(votes, Vote{
		Validator: committee[0].Address,
		ProofHash: pcrypto.SHA256Hash([]byte("proof-A")),
		VRFOutput: committee[0].VRFOutput,
		VRFProof:  committee[0].VRFProof,
	})
	for i := 1; i < len(committee); i++ {
		votes = append(votes, Vote{
			Validator: committee[i].Address,
			ProofHash: pcrypto.SHA256Hash([]byte("proof-B")),
			VRFOutput: committee[i].VRFOutput,
			VRFProof:  committee[i].VRFProof,
		})
	}

	ok, err := qc.CheckQuorum(seed, committee, votes)
	if err != nil {
		t.Fatal(err)
	}

	// Only the first proof (proof-A) has 1 vote which is < quorum
	// The conflicting votes on proof-B should not count toward proof-A's quorum
	// proof-B gets len(committee)-1 votes -- whether quorum passes depends on committee size
	// The point is that the first proof hash locks the consensus
	// Actually: first vote sets consensus to proof-A, so proof-B votes are skipped
	// Total valid votes = 1, which is below quorum (3)
	if ok {
		// This might pass if committee is large enough that proof-B voters are majority
		// But in our implementation, first vote locks the proof hash, so proof-B voters get 0
		// Actually re-reading the code: first valid vote sets consensusProof.
		// Then subsequent votes with different proofHash are skipped (continue).
		// So only 1 valid vote. Should fail.
		t.Error("expected quorum to fail when votes have conflicting proof hashes")
	}
}

func TestQuorumChecker_EmptyCommittee(t *testing.T) {
	qc := NewQuorumChecker(0.6)

	_, err := qc.CheckQuorum(types.Hash{}, nil, nil)
	if err == nil {
		t.Error("expected error for empty committee")
	}
}

func TestQuorumChecker_RequiredVotes(t *testing.T) {
	qc := NewQuorumChecker(0.6)

	tests := []struct {
		size     int
		expected int
	}{
		{5, 3},
		{3, 2},
		{1, 1},
		{10, 6},
		{7, 5},
	}

	for _, tt := range tests {
		got := qc.RequiredVotes(tt.size)
		if got != tt.expected {
			t.Errorf("RequiredVotes(%d) = %d, want %d", tt.size, got, tt.expected)
		}
	}
}
