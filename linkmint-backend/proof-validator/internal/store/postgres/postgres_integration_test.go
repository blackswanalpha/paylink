//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/paylink/proof-validator/internal/domain"
	pgstore "github.com/paylink/proof-validator/internal/store/postgres"
)

var testStore *pgstore.Store

func TestMain(m *testing.M) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("paylink"),
		tcpostgres.WithUsername("paylink"),
		tcpostgres.WithPassword("paylink"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		os.Exit(1)
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		os.Exit(1)
	}
	testStore, err = pgstore.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "new store:", err)
		os.Exit(1)
	}
	if err := testStore.Migrate(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
	code := m.Run()
	testStore.Close()
	_ = testcontainers.TerminateContainer(ctr)
	os.Exit(code)
}

func rec(hash string) domain.ProofRecord {
	now := time.Now().UTC()
	return domain.ProofRecord{
		ProofHash: hash, PayLinkID: "0xpl", Rail: "mpesa", TxID: "t", Amount: 1500,
		Status: domain.StatusReceived, CreatedAt: now, UpdatedAt: now,
	}
}

func TestMigrateIdempotent(t *testing.T) {
	if err := testStore.Migrate(context.Background()); err != nil {
		t.Fatalf("re-migrate should be idempotent: %v", err)
	}
}

func TestInsertAndDuplicate(t *testing.T) {
	ctx := context.Background()
	if err := testStore.InsertProof(ctx, rec("0xdup")); err != nil {
		t.Fatalf("InsertProof: %v", err)
	}
	if err := testStore.InsertProof(ctx, rec("0xdup")); !errors.Is(err, domain.ErrProofExists) {
		t.Fatalf("duplicate insert err = %v, want ErrProofExists", err)
	}
}

func TestMarkBroadcastAndGet(t *testing.T) {
	ctx := context.Background()
	if err := testStore.InsertProof(ctx, rec("0xmark")); err != nil {
		t.Fatalf("InsertProof: %v", err)
	}
	if err := testStore.MarkBroadcast(ctx, "0xmark", "0xtxhash", domain.StatusBroadcast); err != nil {
		t.Fatalf("MarkBroadcast: %v", err)
	}
	got, err := testStore.GetByProofHash(ctx, "0xmark")
	if err != nil {
		t.Fatalf("GetByProofHash: %v", err)
	}
	if got.Status != domain.StatusBroadcast || got.TxHash != "0xtxhash" || got.Amount != 1500 {
		t.Fatalf("record = %+v", got)
	}
}

func TestGetNotFound(t *testing.T) {
	if _, err := testStore.GetByProofHash(context.Background(), "0xmissing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMarkBroadcastNotFound(t *testing.T) {
	if err := testStore.MarkBroadcast(context.Background(), "0xmissing", "0xtx", domain.StatusBroadcast); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPing(t *testing.T) {
	if err := testStore.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
