package fee

// FeeBreakdown contains the calculated fee split.
type FeeBreakdown struct {
	TotalFee        uint64
	ValidatorReward uint64
	TreasuryAmount  uint64
	BurnAmount      uint64
}

// Calculator computes fees for PayLink settlements.
type Calculator struct {
	rateBasisPoints   uint64 // 50 = 0.5%
	validatorSharePct uint64 // 70 = 70%
	treasurySharePct  uint64 // 20 = 20%
	burnSharePct      uint64 // 10 = 10%
	minFee            uint64 // minimum fee floor
}

// NewCalculator creates a fee calculator with the given parameters.
// Shares must sum to 100.
func NewCalculator(rateBPS, validatorPct, treasuryPct, burnPct, minFee uint64) *Calculator {
	return &Calculator{
		rateBasisPoints:   rateBPS,
		validatorSharePct: validatorPct,
		treasurySharePct:  treasuryPct,
		burnSharePct:      burnPct,
		minFee:            minFee,
	}
}

// CalculateFee computes the fee breakdown for a given settlement amount.
// Fee = max(amount * rateBPS / 10000, minFee).
// Split: validatorPct% to validators, treasuryPct% to treasury, burnPct% burned.
// Rounding remainder goes to treasury.
func (c *Calculator) CalculateFee(amount uint64) FeeBreakdown {
	if amount == 0 {
		return FeeBreakdown{}
	}

	totalFee := amount * c.rateBasisPoints / 10000
	if totalFee < c.minFee {
		totalFee = c.minFee
	}

	validatorReward := totalFee * c.validatorSharePct / 100
	burnAmount := totalFee * c.burnSharePct / 100
	// Treasury gets the remainder to absorb rounding
	treasuryAmount := totalFee - validatorReward - burnAmount

	return FeeBreakdown{
		TotalFee:        totalFee,
		ValidatorReward: validatorReward,
		TreasuryAmount:  treasuryAmount,
		BurnAmount:      burnAmount,
	}
}
