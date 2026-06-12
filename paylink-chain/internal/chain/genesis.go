package chain

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// DefaultGenesisTimestamp is the genesis block time for auto-generated dev configs
// (2025-01-01T00:00:00Z). It is a fixed constant — NOT wall-clock time — because the
// genesis hash must be identical on every node that shares a genesis config.
const DefaultGenesisTimestamp int64 = 1735689600

// DefaultGenesis returns a default genesis config for development.
func DefaultGenesis(adminAddr types.Address) *types.GenesisConfig {
	return &types.GenesisConfig{
		ChainID:             "paylink-devnet-1",
		AdminAddress:        adminAddr,
		InitialSupply:       500_000_000_00000000, // 500M with 8 decimals
		MaxSupply:           1_000_000_000_00000000,
		MinimumStake:        10_000_00000000, // 10,000 PLN
		WithdrawalCooldown:  7 * 24 * 3600,   // 7 days in seconds
		RequiredValidations: 3,
		BlockIntervalMs:     1000, // 1 second
		GenesisTimestamp:    DefaultGenesisTimestamp,
		InitialBalances: []types.GenesisBalance{
			{Address: adminAddr, Balance: 500_000_000_00000000},
		},
	}
}

// LoadGenesis loads a genesis config from a JSON file.
func LoadGenesis(path string) (*types.GenesisConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read genesis file: %w", err)
	}
	var genesis types.GenesisConfig
	if err := json.Unmarshal(data, &genesis); err != nil {
		return nil, fmt.Errorf("parse genesis file: %w", err)
	}
	return &genesis, nil
}

// SaveGenesis writes a genesis config to a JSON file.
func SaveGenesis(path string, genesis *types.GenesisConfig) error {
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// CreateGenesisBlock creates the genesis block (height 0) from genesis config.
// The timestamp comes from the config, never from the wall clock — every node
// sharing a genesis config must derive the identical genesis hash.
func CreateGenesisBlock(genesis *types.GenesisConfig, s *state.StateDB) *types.Block {
	stateRoot := s.ComputeStateRoot()

	ts := genesis.GenesisTimestamp
	if ts == 0 {
		ts = DefaultGenesisTimestamp
	}

	block := &types.Block{
		Header: types.BlockHeader{
			Height:       0,
			Timestamp:    ts,
			PreviousHash: types.ZeroHash,
			StateRoot:    stateRoot,
			TxRoot:       types.ZeroHash, // No txs in genesis
			ProposerAddr: genesis.AdminAddress,
		},
		Transactions: nil,
	}

	// Compute block hash
	block.Hash = crypto.SHA256Hash(block.HeaderBytes())

	return block
}
