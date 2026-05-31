package fee

import "testing"

func TestCalculateFee_Standard(t *testing.T) {
	c := NewCalculator(50, 70, 20, 10, 0)
	fb := c.CalculateFee(10000)

	if fb.TotalFee != 50 { // 10000 * 50 / 10000 = 50
		t.Errorf("TotalFee = %d, want 50", fb.TotalFee)
	}
	if fb.ValidatorReward != 35 { // 50 * 70 / 100 = 35
		t.Errorf("ValidatorReward = %d, want 35", fb.ValidatorReward)
	}
	if fb.BurnAmount != 5 { // 50 * 10 / 100 = 5
		t.Errorf("BurnAmount = %d, want 5", fb.BurnAmount)
	}
	if fb.TreasuryAmount != 10 { // 50 - 35 - 5 = 10
		t.Errorf("TreasuryAmount = %d, want 10", fb.TreasuryAmount)
	}
}

func TestCalculateFee_MinFeeFloor(t *testing.T) {
	c := NewCalculator(50, 70, 20, 10, 100)

	// Amount 100 → fee = 100*50/10000 = 0, but min floor = 100
	fb := c.CalculateFee(100)

	if fb.TotalFee != 100 {
		t.Errorf("TotalFee = %d, want 100 (min floor)", fb.TotalFee)
	}
}

func TestCalculateFee_ZeroAmount(t *testing.T) {
	c := NewCalculator(50, 70, 20, 10, 0)
	fb := c.CalculateFee(0)

	if fb.TotalFee != 0 {
		t.Errorf("TotalFee = %d, want 0 for zero amount", fb.TotalFee)
	}
}

func TestCalculateFee_SharesSumToTotal(t *testing.T) {
	c := NewCalculator(50, 70, 20, 10, 0)

	amounts := []uint64{1, 100, 999, 10000, 100000, 1234567}
	for _, amount := range amounts {
		fb := c.CalculateFee(amount)
		sum := fb.ValidatorReward + fb.TreasuryAmount + fb.BurnAmount
		if sum != fb.TotalFee {
			t.Errorf("shares don't sum to total for amount %d: %d + %d + %d = %d, want %d",
				amount, fb.ValidatorReward, fb.TreasuryAmount, fb.BurnAmount, sum, fb.TotalFee)
		}
	}
}

func TestCalculateFee_LargeAmount(t *testing.T) {
	c := NewCalculator(50, 70, 20, 10, 0)
	fb := c.CalculateFee(1_000_000_000)

	// 1B * 50 / 10000 = 5M
	if fb.TotalFee != 5_000_000 {
		t.Errorf("TotalFee = %d, want 5000000", fb.TotalFee)
	}
	if fb.ValidatorReward != 3_500_000 { // 5M * 70% = 3.5M
		t.Errorf("ValidatorReward = %d, want 3500000", fb.ValidatorReward)
	}
}

func TestCalculateFee_Rounding(t *testing.T) {
	c := NewCalculator(50, 70, 20, 10, 0)

	// Amount where fee doesn't divide cleanly
	fb := c.CalculateFee(333)
	// fee = 333 * 50 / 10000 = 1 (integer division)
	if fb.TotalFee != 1 {
		t.Errorf("TotalFee = %d, want 1", fb.TotalFee)
	}
	// 1 * 70/100 = 0, 1 * 10/100 = 0, treasury gets remainder = 1
	sum := fb.ValidatorReward + fb.TreasuryAmount + fb.BurnAmount
	if sum != fb.TotalFee {
		t.Errorf("rounding error: shares sum to %d, want %d", sum, fb.TotalFee)
	}
}
