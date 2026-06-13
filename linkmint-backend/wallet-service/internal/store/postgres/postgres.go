// Package postgres is the production domain.Store backed by PostgreSQL (pgx). Reads are pool
// queries over the indexed `wallet` schema. Each projection write runs in one transaction that joins
// the DbDedupe (work17) mark with the table writes, so an at-least-once redelivery applies exactly
// once. NON-CUSTODIAL (A.1): no balances are held — account_balances caches chain truth; the rest are
// event projections.
package postgres

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	idempotency "github.com/paylink/idempotency-go"

	"github.com/paylink/wallet-service/internal/domain"
)

// DbDedupe scopes (one per consumed chain event).
const (
	scopeTransfer         = "chain.account.transfer"
	scopeStaked           = "chain.validator.staked"
	scopeUnstakeStarted   = "chain.validator.unstake_started"
	scopeUnstakeCompleted = "chain.validator.unstake_completed"
	scopeSlashed          = "chain.validator.slashed"
	scopeRewarded         = "chain.validator.rewarded"
	scopeFeeCollected     = "chain.fee.collected"
	scopeFeeDistributed   = "chain.fee.distributed"
	scopeTokenBurned      = "chain.token.burned"
)

// Store is a pgx-backed domain.Store.
type Store struct {
	pool   *pgxpool.Pool
	dedupe *idempotency.DbDedupe
}

// New connects a pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool, dedupe: idempotency.NewDbDedupe("wallet.processed_events")}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks DB connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// ── helpers ──

func numStr(b *big.Int) string {
	if b == nil {
		return "0"
	}
	return b.String()
}

func parseBig(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}
	return n
}

// runOnce wraps a projection in a transaction joined with the DbDedupe mark. action receives the tx.
func (s *Store) runOnce(ctx context.Context, scope, key string, action func(tx pgx.Tx) error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	applied, err := s.dedupe.RunOnce(ctx, tx, scope, key, func() error { return action(tx) })
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return applied, nil
}

func insertTx(ctx context.Context, tx pgx.Tx, addr, counterparty, direction, kind string, amount *big.Int, txHash string, bh uint64, at time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO wallet.transactions
		   (id, addr, counterparty, direction, kind, amount, tx_hash, block_height, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6::numeric,$7,$8,$9)`,
		uuid.NewString(), addr, counterparty, direction, kind, numStr(amount), txHash, int64(bh), at)
	return err
}

func ensurePosition(ctx context.Context, tx pgx.Tx, addr string, at time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO wallet.staking_positions (addr, updated_at) VALUES ($1,$2)
		 ON CONFLICT (addr) DO NOTHING`, addr, at)
	return err
}

// ── projections ──

// RecordTransfer projects chain.account.transfer into two history rows (out on from, in on to).
func (s *Store) RecordTransfer(ctx context.Context, ev domain.TransferEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.From + ":" + ev.To
	return s.runOnce(ctx, scopeTransfer, key, func(tx pgx.Tx) error {
		if err := insertTx(ctx, tx, ev.From, ev.To, "out", domain.KindTransfer, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt); err != nil {
			return err
		}
		return insertTx(ctx, tx, ev.To, ev.From, "in", domain.KindTransfer, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	})
}

// RecordStaked sets the position's staked amount (authoritative totalStaked) + active flag, and adds
// a stake history row.
func (s *Store) RecordStaked(ctx context.Context, ev domain.StakedEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.Addr
	return s.runOnce(ctx, scopeStaked, key, func(tx pgx.Tx) error {
		if err := ensurePosition(ctx, tx, ev.Addr, ev.OccurredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE wallet.staking_positions
			    SET staked_amount=$2::numeric, is_active=$3, updated_at=$4 WHERE addr=$1`,
			ev.Addr, numStr(ev.TotalStaked), ev.IsActive, ev.OccurredAt); err != nil {
			return err
		}
		return insertTx(ctx, tx, ev.Addr, "", "out", domain.KindStake, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	})
}

// RecordUnstakeStarted moves stake into pending withdrawal and records the cooldown instant.
func (s *Store) RecordUnstakeStarted(ctx context.Context, ev domain.UnstakeStartedEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.Addr
	return s.runOnce(ctx, scopeUnstakeStarted, key, func(tx pgx.Tx) error {
		if err := ensurePosition(ctx, tx, ev.Addr, ev.OccurredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE wallet.staking_positions SET
			    pending_withdrawal = pending_withdrawal + $2::numeric,
			    staked_amount = GREATEST(0, staked_amount - $2::numeric),
			    withdrawable_at = $3,
			    updated_at = $4
			  WHERE addr=$1`,
			ev.Addr, numStr(ev.Amount), ev.WithdrawableAt, ev.OccurredAt); err != nil {
			return err
		}
		return insertTx(ctx, tx, ev.Addr, "", "self", domain.KindUnstakeStart, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	})
}

// RecordUnstakeCompleted clears the pending withdrawal and records the return to balance.
func (s *Store) RecordUnstakeCompleted(ctx context.Context, ev domain.UnstakeCompletedEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.Addr
	return s.runOnce(ctx, scopeUnstakeCompleted, key, func(tx pgx.Tx) error {
		if err := ensurePosition(ctx, tx, ev.Addr, ev.OccurredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE wallet.staking_positions SET
			    pending_withdrawal = GREATEST(0, pending_withdrawal - $2::numeric),
			    updated_at = $3
			  WHERE addr=$1`,
			ev.Addr, numStr(ev.Amount), ev.OccurredAt); err != nil {
			return err
		}
		return insertTx(ctx, tx, ev.Addr, "", "in", domain.KindUnstakeComplete, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	})
}

// RecordSlashed reduces stake to the reported remainder and accrues total_slashed.
func (s *Store) RecordSlashed(ctx context.Context, ev domain.SlashedEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.Addr
	return s.runOnce(ctx, scopeSlashed, key, func(tx pgx.Tx) error {
		if err := ensurePosition(ctx, tx, ev.Addr, ev.OccurredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE wallet.staking_positions SET
			    total_slashed = total_slashed + $2::numeric,
			    staked_amount = $3::numeric,
			    updated_at = $4
			  WHERE addr=$1`,
			ev.Addr, numStr(ev.Amount), numStr(ev.Remaining), ev.OccurredAt); err != nil {
			return err
		}
		return insertTx(ctx, tx, ev.Addr, "", "out", domain.KindSlash, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	})
}

// RecordRewarded sets the authoritative cumulative reward total, appends a reward-history row, and
// records the reward movement.
func (s *Store) RecordRewarded(ctx context.Context, ev domain.RewardedEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.Addr
	return s.runOnce(ctx, scopeRewarded, key, func(tx pgx.Tx) error {
		if err := ensurePosition(ctx, tx, ev.Addr, ev.OccurredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE wallet.staking_positions SET total_rewards=$2::numeric, updated_at=$3 WHERE addr=$1`,
			ev.Addr, numStr(ev.TotalRewards), ev.OccurredAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO wallet.staking_rewards
			   (id, addr, amount, total_rewards, source, tx_hash, block_height, occurred_at)
			 VALUES ($1,$2,$3::numeric,$4::numeric,$5,$6,$7,$8)`,
			uuid.NewString(), ev.Addr, numStr(ev.Amount), numStr(ev.TotalRewards),
			domain.SourceValidatorReward, ev.TxHash, int64(ev.BlockHeight), ev.OccurredAt); err != nil {
			return err
		}
		return insertTx(ctx, tx, ev.Addr, "", "in", domain.KindReward, ev.Amount, ev.TxHash, ev.BlockHeight, ev.OccurredAt)
	})
}

// RecordFeeCollected accrues the per-settlement fee deltas onto the treasury aggregate (total_burned
// is owned by RecordTokenBurned to avoid double counting).
func (s *Store) RecordFeeCollected(ctx context.Context, ev domain.FeeCollectedEvent) (bool, error) {
	key := ev.TxHash
	if key == "" {
		key = "fee:" + ev.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	return s.runOnce(ctx, scopeFeeCollected, key, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE wallet.treasury_stats SET
			    fees_collected    = fees_collected    + $1::numeric,
			    validator_rewards = validator_rewards + $2::numeric,
			    treasury_amount   = treasury_amount   + $3::numeric,
			    chain_height      = GREATEST(chain_height, $4),
			    updated_at        = $5
			  WHERE id = 1`,
			numStr(ev.TotalFee), numStr(ev.ValidatorShare), numStr(ev.TreasuryShare), int64(ev.BlockHeight), ev.OccurredAt)
		return err
	})
}

// RecordFeeDistributed appends a fee_share reward-history row for the validator. It does not touch
// the position's total_rewards (that mirrors the chain's TotalRewards, bumped only by validator.rewarded).
func (s *Store) RecordFeeDistributed(ctx context.Context, ev domain.FeeDistributedEvent) (bool, error) {
	key := ev.TxHash + ":" + ev.Validator
	return s.runOnce(ctx, scopeFeeDistributed, key, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO wallet.staking_rewards
			   (id, addr, amount, total_rewards, source, tx_hash, block_height, occurred_at)
			 VALUES ($1,$2,$3::numeric,0,$4,$5,$6,$7)`,
			uuid.NewString(), ev.Validator, numStr(ev.Amount), domain.SourceFeeShare,
			ev.TxHash, int64(ev.BlockHeight), ev.OccurredAt)
		return err
	})
}

// RecordTokenBurned sets the authoritative cumulative burn total on the treasury aggregate.
func (s *Store) RecordTokenBurned(ctx context.Context, ev domain.TokenBurnedEvent) (bool, error) {
	key := ev.TxHash
	if key == "" {
		key = "burn:" + ev.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	return s.runOnce(ctx, scopeTokenBurned, key, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE wallet.treasury_stats SET
			    total_burned = $1::numeric,
			    chain_height = GREATEST(chain_height, $2),
			    updated_at   = $3
			  WHERE id = 1`,
			numStr(ev.TotalBurned), int64(ev.BlockHeight), ev.OccurredAt)
		return err
	})
}

// ── reads ──

// GetAccountCache returns the cached balance row for addr (ok=false when none exists).
func (s *Store) GetAccountCache(ctx context.Context, addr string) (domain.Account, bool, error) {
	var (
		balance     string
		nonce       int64
		blockHeight int64
		fetchedAt   time.Time
	)
	err := s.pool.QueryRow(ctx,
		`SELECT balance::text, nonce, block_height, fetched_at FROM wallet.account_balances WHERE addr=$1`, addr).
		Scan(&balance, &nonce, &blockHeight, &fetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	return domain.Account{
		Addr: addr, Balance: parseBig(balance), Nonce: uint64(nonce),
		BlockHeight: uint64(blockHeight), FetchedAt: fetchedAt,
	}, true, nil
}

// UpsertAccountCache writes the read-through balance row.
func (s *Store) UpsertAccountCache(ctx context.Context, a domain.Account) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO wallet.account_balances (addr, balance, nonce, block_height, source, fetched_at)
		 VALUES ($1,$2::numeric,$3,$4,'rpc',$5)
		 ON CONFLICT (addr) DO UPDATE SET
		    balance=EXCLUDED.balance, nonce=EXCLUDED.nonce,
		    block_height=EXCLUDED.block_height, source=EXCLUDED.source, fetched_at=EXCLUDED.fetched_at`,
		a.Addr, numStr(a.Balance), int64(a.Nonce), int64(a.BlockHeight), a.FetchedAt)
	return err
}

// ListTransactions returns a newest-first page of an address's history with a keyset cursor.
func (s *Store) ListTransactions(ctx context.Context, addr string, limit int, cursor string) ([]domain.Transaction, string, error) {
	limit = domain.ClampLimit(limit)
	rows, err := s.queryPage(ctx,
		`SELECT id, counterparty, direction, kind, amount::text, tx_hash, block_height, occurred_at
		   FROM wallet.transactions WHERE addr=$1`, addr, limit, cursor)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []domain.Transaction
	for rows.Next() {
		var (
			t           domain.Transaction
			amount      string
			blockHeight int64
		)
		if err := rows.Scan(&t.ID, &t.Counterparty, &t.Direction, &t.Kind, &amount, &t.TxHash, &blockHeight, &t.OccurredAt); err != nil {
			return nil, "", err
		}
		t.Addr = addr
		t.Amount = parseBig(amount)
		t.BlockHeight = uint64(blockHeight)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == limit {
		last := out[len(out)-1]
		next = domain.EncodeCursor(last.BlockHeight, last.ID)
	}
	return out, next, nil
}

// ListRewards returns a newest-first page of an address's reward history with a keyset cursor.
func (s *Store) ListRewards(ctx context.Context, addr string, limit int, cursor string) ([]domain.Reward, string, error) {
	limit = domain.ClampLimit(limit)
	rows, err := s.queryPage(ctx,
		`SELECT id, amount::text, total_rewards::text, source, tx_hash, block_height, occurred_at
		   FROM wallet.staking_rewards WHERE addr=$1`, addr, limit, cursor)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []domain.Reward
	for rows.Next() {
		var (
			r            domain.Reward
			amount       string
			totalRewards string
			blockHeight  int64
		)
		if err := rows.Scan(&r.ID, &amount, &totalRewards, &r.Source, &r.TxHash, &blockHeight, &r.OccurredAt); err != nil {
			return nil, "", err
		}
		r.Addr = addr
		r.Amount = parseBig(amount)
		r.TotalRewards = parseBig(totalRewards)
		r.BlockHeight = uint64(blockHeight)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == limit {
		last := out[len(out)-1]
		next = domain.EncodeCursor(last.BlockHeight, last.ID)
	}
	return out, next, nil
}

// queryPage runs a keyset-paginated SELECT (base WHERE already constrains addr as $1), appending the
// cursor predicate and the ORDER BY / LIMIT. The base query's first column must be the id.
func (s *Store) queryPage(ctx context.Context, base, addr string, limit int, cursor string) (pgx.Rows, error) {
	if bh, id, ok := domain.DecodeCursor(cursor); ok {
		return s.pool.Query(ctx,
			base+` AND (block_height < $2 OR (block_height = $2 AND id < $3))
			       ORDER BY block_height DESC, id DESC LIMIT $4`,
			addr, int64(bh), id, limit)
	}
	return s.pool.Query(ctx,
		base+` ORDER BY block_height DESC, id DESC LIMIT $2`, addr, limit)
}

// GetPosition returns an address's staking position (ok=false when it has never staked).
func (s *Store) GetPosition(ctx context.Context, addr string) (domain.Position, bool, error) {
	var (
		staked     string
		pending    string
		rewards    string
		slashed    string
		withdrawAt *time.Time
		p          domain.Position
	)
	err := s.pool.QueryRow(ctx,
		`SELECT staked_amount::text, pending_withdrawal::text, total_rewards::text, total_slashed::text,
		        withdrawable_at, is_active, updated_at
		   FROM wallet.staking_positions WHERE addr=$1`, addr).
		Scan(&staked, &pending, &rewards, &slashed, &withdrawAt, &p.IsActive, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Position{}, false, nil
	}
	if err != nil {
		return domain.Position{}, false, err
	}
	p.Addr = addr
	p.StakedAmount = parseBig(staked)
	p.PendingWithdrawal = parseBig(pending)
	p.TotalRewards = parseBig(rewards)
	p.TotalSlashed = parseBig(slashed)
	p.WithdrawableAt = withdrawAt
	return p, true, nil
}

// GetTreasuryStats returns the single-row treasury aggregate.
func (s *Store) GetTreasuryStats(ctx context.Context) (domain.TreasuryStats, error) {
	var (
		totalSupply, maxSupply, totalBurned, fees, rewards, treasury string
		chainHeight                                                  int64
		t                                                            domain.TreasuryStats
	)
	err := s.pool.QueryRow(ctx,
		`SELECT total_supply::text, max_supply::text, total_burned::text, fees_collected::text,
		        validator_rewards::text, treasury_amount::text, chain_height, updated_at
		   FROM wallet.treasury_stats WHERE id=1`).
		Scan(&totalSupply, &maxSupply, &totalBurned, &fees, &rewards, &treasury, &chainHeight, &t.UpdatedAt)
	if err != nil {
		return domain.TreasuryStats{}, err
	}
	t.TotalSupply = parseBig(totalSupply)
	t.MaxSupply = parseBig(maxSupply)
	t.TotalBurned = parseBig(totalBurned)
	t.FeesCollected = parseBig(fees)
	t.ValidatorRewards = parseBig(rewards)
	t.TreasuryAmount = parseBig(treasury)
	t.ChainHeight = uint64(chainHeight)
	return t, nil
}
