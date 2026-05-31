package fee

import (
	"fmt"

	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// Distributor distributes fees to validators, treasury, and burns tokens.
type Distributor struct {
	state           *state.StateDB
	treasuryAddress types.Address
}

// NewDistributor creates a fee distributor.
func NewDistributor(s *state.StateDB, treasuryAddr types.Address) *Distributor {
	return &Distributor{
		state:           s,
		treasuryAddress: treasuryAddr,
	}
}

// DistributeFees mints PLN rewards to voting validators, credits treasury, and burns tokens.
// Validator rewards are split proportionally by stake weight among voters.
func (d *Distributor) DistributeFees(fee FeeBreakdown, voters []types.Address) ([]ValidatorPayout, error) {
	if fee.TotalFee == 0 {
		return nil, nil
	}

	// Calculate total stake of voters for proportional distribution
	totalVoterStake := uint64(0)
	voterStakes := make(map[types.Address]uint64)
	for _, addr := range voters {
		v := d.state.GetValidator(addr)
		if v != nil && v.IsActive {
			voterStakes[addr] = v.StakedAmount
			totalVoterStake += v.StakedAmount
		}
	}

	if totalVoterStake == 0 {
		return nil, fmt.Errorf("no active voters with stake")
	}

	// Distribute validator rewards proportional to stake
	var payouts []ValidatorPayout
	distributed := uint64(0)
	voterList := make([]types.Address, 0, len(voterStakes))
	for addr := range voterStakes {
		voterList = append(voterList, addr)
	}

	for i, addr := range voterList {
		var reward uint64
		if i == len(voterList)-1 {
			// Last voter gets remainder to avoid rounding loss
			reward = fee.ValidatorReward - distributed
		} else {
			reward = fee.ValidatorReward * voterStakes[addr] / totalVoterStake
		}
		distributed += reward

		if reward > 0 {
			if err := d.state.MintTokens(addr, reward); err != nil {
				return nil, fmt.Errorf("mint validator reward: %w", err)
			}
			payouts = append(payouts, ValidatorPayout{
				Validator: addr,
				Amount:    reward,
			})
		}
	}

	// Credit treasury
	if fee.TreasuryAmount > 0 {
		if err := d.state.MintTokens(d.treasuryAddress, fee.TreasuryAmount); err != nil {
			return nil, fmt.Errorf("mint treasury: %w", err)
		}
	}

	// Burn tokens
	if fee.BurnAmount > 0 {
		d.state.BurnTokens(fee.BurnAmount)
	}

	return payouts, nil
}

// ValidatorPayout represents a reward payment to a validator.
type ValidatorPayout struct {
	Validator types.Address
	Amount    uint64
}
