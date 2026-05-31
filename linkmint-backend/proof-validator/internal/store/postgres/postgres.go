// Package postgres is the production domain.Store backed by PostgreSQL (pgx). proof_hash is the
// primary key, so a duplicate proof submission collides locally (complementing on-chain A.7).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paylink/proof-validator/internal/domain"
)

// Store is a pgx-backed domain.Store.
type Store struct {
	pool *pgxpool.Pool
}

// New connects a pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks DB connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const selectColumns = `proof_hash, paylink_id, rail, tx_id, amount, status, tx_hash, created_at, updated_at`

// InsertProof inserts a proof record, mapping the unique proof_hash violation to ErrProofExists.
func (s *Store) InsertProof(ctx context.Context, r domain.ProofRecord) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO proof_validator.proofs (proof_hash, paylink_id, rail, tx_id, amount, status, tx_hash, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		r.ProofHash, r.PayLinkID, r.Rail, r.TxID, int64(r.Amount), r.Status, r.TxHash, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrProofExists
		}
		return err
	}
	return nil
}

// MarkBroadcast records the settlement tx hash and advances the status. Returns ErrNotFound if no
// row matches the proof hash.
func (s *Store) MarkBroadcast(ctx context.Context, proofHash, txHash, status string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE proof_validator.proofs SET tx_hash=$1, status=$2, updated_at=now() WHERE proof_hash=$3`,
		txHash, status, proofHash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetByProofHash returns the record, or domain.ErrNotFound.
func (s *Store) GetByProofHash(ctx context.Context, proofHash string) (domain.ProofRecord, error) {
	var (
		r      domain.ProofRecord
		amount int64
	)
	err := s.pool.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM proof_validator.proofs WHERE proof_hash=$1`, proofHash).
		Scan(&r.ProofHash, &r.PayLinkID, &r.Rail, &r.TxID, &amount, &r.Status, &r.TxHash, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProofRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ProofRecord{}, err
	}
	r.Amount = uint64(amount)
	return r, nil
}
