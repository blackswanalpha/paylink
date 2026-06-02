package ledger

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Direction is the side of a ledger leg: DR (debit) or CR (credit).
type Direction string

const (
	DR Direction = "DR"
	CR Direction = "CR"
)

// flip returns the opposite direction (used by Reverse).
func flip(d Direction) Direction {
	if d == CR {
		return DR
	}
	return CR
}

// Leg is one half-entry of a posting: a single account movement. Amount is in minor units
// (NUMERIC(38,0)) and is always strictly positive — the Direction carries the sign.
type Leg struct {
	Account   string
	Direction Direction
	Amount    *big.Int
	Currency  string
}

// PostingInput is a balanced set of legs written atomically under one entry_group. PLID and Note,
// when set, are stamped on every leg of the group (a posting records one business event).
type PostingInput struct {
	EntryGroup *uuid.UUID // optional; a v4 UUID is generated when nil
	Entries    []Leg
	PLID       string
	Note       string
}

// Entry is a persisted ledger row (one leg) as read back from the table.
type Entry struct {
	ID         int64
	EntryGroup uuid.UUID
	Account    string
	Direction  Direction
	Amount     *big.Int
	Currency   string
	PLID       string
	Note       string
	CreatedAt  time.Time
}

// validate enforces the double-entry invariant (A.6) before any write: every leg is well-formed,
// the group has at least one DR and one CR, and DR total == CR total for each currency. A balanced
// multi-currency group is allowed (each currency must balance on its own).
func validate(entries []Leg) error {
	if len(entries) < 2 {
		return fmt.Errorf("%w: a posting needs at least one DR and one CR (got %d legs)", ErrUnbalanced, len(entries))
	}

	type sums struct{ dr, cr *big.Int }
	perCurrency := map[string]*sums{}
	var haveDR, haveCR bool

	for i, leg := range entries {
		if leg.Direction != DR && leg.Direction != CR {
			return fmt.Errorf("%w: leg %d has invalid direction %q", ErrInvalidLeg, i, leg.Direction)
		}
		if strings.TrimSpace(leg.Account) == "" {
			return fmt.Errorf("%w: leg %d has an empty account", ErrInvalidLeg, i)
		}
		if strings.TrimSpace(leg.Currency) == "" {
			return fmt.Errorf("%w: leg %d has an empty currency", ErrInvalidLeg, i)
		}
		if leg.Amount == nil || leg.Amount.Sign() <= 0 {
			return fmt.Errorf("%w: leg %d amount must be a positive integer", ErrInvalidLeg, i)
		}

		s := perCurrency[leg.Currency]
		if s == nil {
			s = &sums{dr: new(big.Int), cr: new(big.Int)}
			perCurrency[leg.Currency] = s
		}
		if leg.Direction == DR {
			s.dr.Add(s.dr, leg.Amount)
			haveDR = true
		} else {
			s.cr.Add(s.cr, leg.Amount)
			haveCR = true
		}
	}

	if !haveDR || !haveCR {
		return fmt.Errorf("%w: a posting needs at least one DR and one CR", ErrUnbalanced)
	}
	for ccy, s := range perCurrency {
		if s.dr.Cmp(s.cr) != 0 {
			return fmt.Errorf("%w: currency %s is unbalanced (DR=%s CR=%s)", ErrUnbalanced, ccy, s.dr, s.cr)
		}
	}
	return nil
}
