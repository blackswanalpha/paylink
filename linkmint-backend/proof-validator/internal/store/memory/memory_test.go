package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paylink/proof-validator/internal/domain"
	"github.com/paylink/proof-validator/internal/store/memory"
)

func sample(hash string) domain.ProofRecord {
	now := time.Now().UTC()
	return domain.ProofRecord{
		ProofHash: hash, PayLinkID: "0xpl", Rail: "mpesa", TxID: "t", Amount: 1500,
		Status: domain.StatusReceived, CreatedAt: now, UpdatedAt: now,
	}
}

func TestMemory_InsertGetDuplicate(t *testing.T) {
	ctx := context.Background()
	s := memory.New()

	if err := s.InsertProof(ctx, sample("0xa")); err != nil {
		t.Fatalf("InsertProof: %v", err)
	}
	if err := s.InsertProof(ctx, sample("0xa")); !errors.Is(err, domain.ErrProofExists) {
		t.Fatalf("duplicate = %v, want ErrProofExists", err)
	}
	got, err := s.GetByProofHash(ctx, "0xa")
	if err != nil || got.ProofHash != "0xa" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestMemory_MarkBroadcast(t *testing.T) {
	ctx := context.Background()
	s := memory.New()
	_ = s.InsertProof(ctx, sample("0xb"))
	if err := s.MarkBroadcast(ctx, "0xb", "0xtx", domain.StatusBroadcast); err != nil {
		t.Fatalf("MarkBroadcast: %v", err)
	}
	got, _ := s.GetByProofHash(ctx, "0xb")
	if got.Status != domain.StatusBroadcast || got.TxHash != "0xtx" {
		t.Fatalf("record = %+v", got)
	}
}

func TestMemory_NotFound(t *testing.T) {
	ctx := context.Background()
	s := memory.New()
	if _, err := s.GetByProofHash(ctx, "0xnope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get err = %v, want ErrNotFound", err)
	}
	if err := s.MarkBroadcast(ctx, "0xnope", "0xtx", domain.StatusBroadcast); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("MarkBroadcast err = %v, want ErrNotFound", err)
	}
	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
