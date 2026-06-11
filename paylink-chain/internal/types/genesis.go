package types

// GenesisConfig defines the initial state of the chain.
type GenesisConfig struct {
	ChainID             string           `json:"chainId"`
	AdminAddress        Address          `json:"adminAddress"`
	InitialSupply       uint64           `json:"initialSupply"`
	MaxSupply           uint64           `json:"maxSupply"`
	MinimumStake        uint64           `json:"minimumStake"`
	WithdrawalCooldown  int64            `json:"withdrawalCooldown"` // seconds
	RequiredValidations uint64           `json:"requiredValidations"`
	BlockIntervalMs     int64            `json:"blockIntervalMs"`
	GenesisTimestamp    int64            `json:"genesisTimestamp"` // Unix seconds; part of the genesis hash, must match across nodes
	InitialBalances     []GenesisBalance `json:"initialBalances"`

	// Phase 2: Committee and fee parameters
	TargetCommitteeSize  int     `json:"targetCommitteeSize,omitempty"` // default 5
	QuorumFraction       float64 `json:"quorumFraction,omitempty"`      // default 0.6 (3 of 5)
	TreasuryAddress      Address `json:"treasuryAddress,omitempty"`
	FeeRateBasisPoints   uint64  `json:"feeRateBasisPoints,omitempty"`   // 50 = 0.5%
	ValidatorRewardShare uint64  `json:"validatorRewardShare,omitempty"` // 70 = 70%
	TreasuryShare        uint64  `json:"treasuryShare,omitempty"`        // 20 = 20%
	BurnShare            uint64  `json:"burnShare,omitempty"`            // 10 = 10%
}

// GenesisBalance defines a pre-funded account in genesis.
type GenesisBalance struct {
	Address Address `json:"address"`
	Balance uint64  `json:"balance"`
}
