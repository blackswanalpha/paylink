//go:build integration

// Integration tests for the pgx-backed ledger helpers. Run with: go test -tags=integration ./...
// Requires a Docker daemon (testcontainers spins an ephemeral postgres:16).
package ledger

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("paylink"),
		tcpostgres.WithUsername("paylink"),
		tcpostgres.WithPassword("paylink"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	m := NewMigrator(pool)
	if err := m.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := m.Migrate(ctx); err != nil {
		t.Fatalf("re-migrate (must be idempotent): %v", err)
	}
	return pool
}

func mustPostBalanced(t *testing.T, db DBTX) uuid.UUID {
	t.Helper()
	g, err := Post(context.Background(), db, PostingInput{
		Entries: []Leg{
			{Account: "paylink:PLK1", Direction: DR, Amount: big.NewInt(1000), Currency: "PLN"},
			{Account: "treasury", Direction: CR, Amount: big.NewInt(1000), Currency: "PLN"},
		},
		PLID: "PLK1",
		Note: "settlement",
	})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return g
}

func countRows(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM ledger.ledger_entries`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestPostBalancedPersistsGroup(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	group := mustPostBalanced(t, pool)

	entries, err := EntriesByGroup(ctx, pool, group)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 legs, got %d", len(entries))
	}
	for _, e := range entries {
		if e.EntryGroup != group {
			t.Fatalf("entry_group mismatch: %s != %s", e.EntryGroup, group)
		}
		if e.PLID != "PLK1" {
			t.Fatalf("pl_id = %q, want PLK1", e.PLID)
		}
		if e.Amount.Cmp(big.NewInt(1000)) != 0 {
			t.Fatalf("amount = %s, want 1000", e.Amount)
		}
		if e.CreatedAt.IsZero() {
			t.Fatal("created_at not populated")
		}
	}
}

func TestPostUnbalancedRejectedNoRows(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	_, err := Post(ctx, pool, PostingInput{Entries: []Leg{
		{Account: "a", Direction: DR, Amount: big.NewInt(100), Currency: "PLN"},
		{Account: "b", Direction: CR, Amount: big.NewInt(90), Currency: "PLN"},
	}})
	if !errors.Is(err, ErrUnbalanced) {
		t.Fatalf("want ErrUnbalanced, got %v", err)
	}
	if n := countRows(t, pool); n != 0 {
		t.Fatalf("unbalanced post wrote %d rows, want 0", n)
	}
}

func TestAppendOnlyUpdateDeleteRejected(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	group := mustPostBalanced(t, pool)

	assertAppendOnly := func(err error, op string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s on append-only ledger should be rejected", op)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
			t.Fatalf("%s: want raise_exception (P0001), got %v", op, err)
		}
	}

	_, uerr := pool.Exec(ctx, `UPDATE ledger.ledger_entries SET amount = amount + 1 WHERE entry_group=$1::uuid`, group.String())
	assertAppendOnly(uerr, "UPDATE")

	_, derr := pool.Exec(ctx, `DELETE FROM ledger.ledger_entries WHERE entry_group=$1::uuid`, group.String())
	assertAppendOnly(derr, "DELETE")

	// History is intact after the rejected mutations.
	if n := countRows(t, pool); n != 2 {
		t.Fatalf("append-only table altered: %d rows, want 2", n)
	}
}

func TestReverseRoundTrip(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	group := mustPostBalanced(t, pool)

	rev, err := Reverse(ctx, pool, group, "")
	if err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if rev == group {
		t.Fatal("reversal must be a new entry_group")
	}

	// Every touched account nets to zero after the reversal.
	for _, acct := range []string{"paylink:PLK1", "treasury"} {
		bal, err := Balance(ctx, pool, acct, "PLN")
		if err != nil {
			t.Fatal(err)
		}
		if bal.Sign() != 0 {
			t.Fatalf("balance %s = %s, want 0 after reversal", acct, bal)
		}
	}

	// Original group is untouched; reversed group has flipped directions.
	if orig, _ := EntriesByGroup(ctx, pool, group); len(orig) != 2 {
		t.Fatalf("original group altered: %d legs", len(orig))
	}
	revEntries, _ := EntriesByGroup(ctx, pool, rev)
	if len(revEntries) != 2 {
		t.Fatalf("reversal has %d legs, want 2", len(revEntries))
	}
	for _, e := range revEntries {
		if e.Account == "treasury" && e.Direction != DR {
			t.Fatalf("treasury should be DR in the reversal, got %s", e.Direction)
		}
		if e.Account == "paylink:PLK1" && e.Direction != CR {
			t.Fatalf("paylink should be CR in the reversal, got %s", e.Direction)
		}
	}

	// Reversing a non-existent group is rejected.
	if _, err := Reverse(ctx, pool, uuid.New(), ""); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("want ErrGroupNotFound, got %v", err)
	}
}

func TestBalanceReadsAndIsBalanced(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()

	// Fee settlement posting (0.5% fee split 70/20/10 — reuses the chain fee semantics).
	if _, err := Post(ctx, pool, PostingInput{
		Entries: []Leg{
			{Account: "paylink:PLK9", Direction: DR, Amount: big.NewInt(1000), Currency: "PLN"},
			{Account: "validator:0xabc", Direction: CR, Amount: big.NewInt(700), Currency: "PLN"},
			{Account: "treasury", Direction: CR, Amount: big.NewInt(200), Currency: "PLN"},
			{Account: "burn", Direction: CR, Amount: big.NewInt(100), Currency: "PLN"},
		},
		PLID: "PLK9",
		Note: "fee 0.5% split 70/20/10",
	}); err != nil {
		t.Fatalf("post: %v", err)
	}

	if bal, _ := Balance(ctx, pool, "treasury", "PLN"); bal.Cmp(big.NewInt(200)) != 0 {
		t.Fatalf("treasury balance = %s, want 200 (ΣCR−ΣDR)", bal)
	}
	if bal, _ := Balance(ctx, pool, "paylink:PLK9", "PLN"); bal.Cmp(big.NewInt(-1000)) != 0 {
		t.Fatalf("paylink balance = %s, want -1000", bal)
	}

	ok, err := IsBalanced(ctx, pool, "PLN")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("PLN ledger should net to zero")
	}

	if byPL, _ := EntriesByPLID(ctx, pool, "PLK9", 10); len(byPL) != 4 {
		t.Fatalf("EntriesByPLID = %d, want 4", len(byPL))
	}
	if byAcct, _ := EntriesByAccount(ctx, pool, "treasury", 10); len(byAcct) != 1 {
		t.Fatalf("EntriesByAccount = %d, want 1", len(byAcct))
	}
}

func TestPostJoinsCallerTx(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	legs := []Leg{
		{Account: "a", Direction: DR, Amount: big.NewInt(5), Currency: "PLN"},
		{Account: "b", Direction: CR, Amount: big.NewInt(5), Currency: "PLN"},
	}

	// Rollback path: a post on the caller's tx vanishes when the caller rolls back.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Post(ctx, tx, PostingInput{Entries: legs}); err != nil {
		t.Fatalf("post on tx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool); n != 0 {
		t.Fatalf("rolled-back post left %d rows", n)
	}

	// Commit path: the legs persist with the caller's transaction.
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Post(ctx, tx2, PostingInput{Entries: legs}); err != nil {
		t.Fatalf("post on tx2: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, pool); n != 2 {
		t.Fatalf("committed post wrote %d rows, want 2", n)
	}
}

func TestBigAmountRoundTripThroughDB(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	// 38 digits — larger than int64's max (~9.2e18); proves the NUMERIC(38,0)/::text/*big.Int path.
	huge, _ := new(big.Int).SetString("12345678901234567890123456789012345678", 10)
	group, err := Post(ctx, pool, PostingInput{Entries: []Leg{
		{Account: "a", Direction: DR, Amount: huge, Currency: "PLN"},
		{Account: "b", Direction: CR, Amount: huge, Currency: "PLN"},
	}})
	if err != nil {
		t.Fatalf("post huge: %v", err)
	}
	entries, _ := EntriesByGroup(ctx, pool, group)
	for _, e := range entries {
		if e.Amount.Cmp(huge) != 0 {
			t.Fatalf("amount round-trip: got %s, want %s", e.Amount, huge)
		}
	}
}
