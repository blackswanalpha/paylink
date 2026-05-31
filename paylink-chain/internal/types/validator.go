package types

// ValidatorInfo represents a validator's staking state.
type ValidatorInfo struct {
	Address           Address `json:"address"`
	StakedAmount      uint64  `json:"stakedAmount"`
	PendingWithdrawal uint64  `json:"pendingWithdrawal"`
	WithdrawableAt    int64   `json:"withdrawableAt"` // Unix timestamp
	TotalSlashed      uint64  `json:"totalSlashed"`
	TotalRewards      uint64  `json:"totalRewards"`
	IsActive          bool    `json:"isActive"`
	JoinedAt          int64   `json:"joinedAt"` // Unix timestamp

	// Phase 2: VRF and tiering
	VRFPublicKey []byte `json:"vrfPublicKey,omitempty"` // ED25519 VRF public key (32 bytes)
	Tier         string `json:"tier,omitempty"`          // "LIGHT", "STANDARD", "SENTINEL"
}

// Validator tier constants.
const (
	TierLight    = "LIGHT"
	TierStandard = "STANDARD"
	TierSentinel = "SENTINEL"
)

// ValidatorStakeInfo is a lightweight view of a validator for committee selection.
type ValidatorStakeInfo struct {
	Address      Address `json:"address"`
	StakedAmount uint64  `json:"stakedAmount"`
	VRFPublicKey []byte  `json:"vrfPublicKey"`
}
