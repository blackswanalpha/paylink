package state

import (
	"encoding/json"
	"sort"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/types"
)

// ComputeStateRoot computes a deterministic state root from all state.
// StateRoot = SHA256(accountTreeRoot || paylinkTreeRoot || validatorTreeRoot ||
// proofTreeRoot || operatorTreeRoot || evidenceTreeRoot)
func (s *StateDB) ComputeStateRoot() types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accountRoot := computeAccountRoot(s.accounts)
	paylinkRoot := computePayLinkRoot(s.paylinks)
	validatorRoot := computeValidatorRoot(s.validators)
	proofRoot := computeProofRoot(s.usedProofs)
	operatorRoot := computeOperatorApprovalRoot(s.operatorApprovals)
	evidenceRoot := computeProofRoot(s.processedEvidence)

	return crypto.CombineHashes(accountRoot, paylinkRoot, validatorRoot, proofRoot, operatorRoot, evidenceRoot)
}

// computeAccountRoot computes a sorted Merkle root of all accounts.
func computeAccountRoot(accounts map[types.Address]*types.Account) types.Hash {
	if len(accounts) == 0 {
		return types.ZeroHash
	}

	// Sort addresses
	addrs := make([]types.Address, 0, len(accounts))
	for addr := range accounts {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool {
		return string(addrs[i][:]) < string(addrs[j][:])
	})

	// Build leaf hashes
	leaves := make([]types.Hash, len(addrs))
	for i, addr := range addrs {
		data, _ := json.Marshal(struct {
			Address types.Address  `json:"a"`
			Account *types.Account `json:"v"`
		}{addr, accounts[addr]})
		leaves[i] = crypto.SHA256Hash(data)
	}

	return merkleRoot(leaves)
}

// computePayLinkRoot computes a sorted Merkle root of all paylinks.
func computePayLinkRoot(paylinks map[types.Hash]*types.PayLink) types.Hash {
	if len(paylinks) == 0 {
		return types.ZeroHash
	}

	ids := make([]types.Hash, 0, len(paylinks))
	for id := range paylinks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return string(ids[i][:]) < string(ids[j][:])
	})

	leaves := make([]types.Hash, len(ids))
	for i, id := range ids {
		data, _ := json.Marshal(paylinks[id])
		leaves[i] = crypto.SHA256Hash(data)
	}

	return merkleRoot(leaves)
}

// computeValidatorRoot computes a sorted Merkle root of all validators.
func computeValidatorRoot(validators map[types.Address]*types.ValidatorInfo) types.Hash {
	if len(validators) == 0 {
		return types.ZeroHash
	}

	addrs := make([]types.Address, 0, len(validators))
	for addr := range validators {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool {
		return string(addrs[i][:]) < string(addrs[j][:])
	})

	leaves := make([]types.Hash, len(addrs))
	for i, addr := range addrs {
		data, _ := json.Marshal(validators[addr])
		leaves[i] = crypto.SHA256Hash(data)
	}

	return merkleRoot(leaves)
}

// computeProofRoot computes a sorted Merkle root of used proofs.
func computeProofRoot(proofs map[types.Hash]bool) types.Hash {
	if len(proofs) == 0 {
		return types.ZeroHash
	}

	hashes := make([]types.Hash, 0, len(proofs))
	for h := range proofs {
		hashes = append(hashes, h)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return string(hashes[i][:]) < string(hashes[j][:])
	})

	leaves := make([]types.Hash, len(hashes))
	for i, h := range hashes {
		leaves[i] = crypto.SHA256Hash(h[:])
	}

	return merkleRoot(leaves)
}

// computeOperatorApprovalRoot computes a sorted Merkle root of operator approvals.
func computeOperatorApprovalRoot(approvals map[operatorKey]bool) types.Hash {
	if len(approvals) == 0 {
		return types.ZeroHash
	}

	type entry struct {
		Owner    types.Address `json:"owner"`
		Operator types.Address `json:"operator"`
	}
	entries := make([]entry, 0, len(approvals))
	for k := range approvals {
		entries = append(entries, entry{Owner: k.Owner, Operator: k.Operator})
	}
	sort.Slice(entries, func(i, j int) bool {
		if string(entries[i].Owner[:]) != string(entries[j].Owner[:]) {
			return string(entries[i].Owner[:]) < string(entries[j].Owner[:])
		}
		return string(entries[i].Operator[:]) < string(entries[j].Operator[:])
	})

	leaves := make([]types.Hash, len(entries))
	for i, e := range entries {
		data, _ := json.Marshal(e)
		leaves[i] = crypto.SHA256Hash(data)
	}
	return merkleRoot(leaves)
}

// merkleRoot computes a binary Merkle tree root from leaves.
func merkleRoot(leaves []types.Hash) types.Hash {
	if len(leaves) == 0 {
		return types.ZeroHash
	}
	if len(leaves) == 1 {
		return leaves[0]
	}

	// Pad to even number
	for len(leaves)%2 != 0 {
		leaves = append(leaves, leaves[len(leaves)-1])
	}

	var next []types.Hash
	for i := 0; i < len(leaves); i += 2 {
		combined := append(leaves[i][:], leaves[i+1][:]...)
		next = append(next, crypto.SHA256Hash(combined))
	}

	return merkleRoot(next)
}
