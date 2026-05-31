package chain_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/paylink/proof-validator/internal/chain"
)

// stubNonceReader returns a fixed chain nonce and counts calls.
type stubNonceReader struct {
	value uint64
	err   error
}

func (s *stubNonceReader) GetNonce(context.Context, string) (uint64, error) {
	return s.value, s.err
}

func TestNonce_AdvancesOnCommit(t *testing.T) {
	nm := chain.NewNonceManager(&stubNonceReader{value: 0})
	ctx := context.Background()
	for want := uint64(0); want < 3; want++ {
		n, commit, err := nm.Reserve(ctx, "0xa")
		if err != nil {
			t.Fatalf("Reserve: %v", err)
		}
		if n != want {
			t.Fatalf("nonce = %d, want %d", n, want)
		}
		commit(true)
	}
}

func TestNonce_NoGapOnFailure(t *testing.T) {
	nm := chain.NewNonceManager(&stubNonceReader{value: 0})
	ctx := context.Background()

	n, commit, _ := nm.Reserve(ctx, "0xa")
	if n != 0 {
		t.Fatalf("first nonce = %d, want 0", n)
	}
	commit(false) // submission failed → do not advance

	n, commit, _ = nm.Reserve(ctx, "0xa")
	if n != 0 {
		t.Fatalf("after failed submit, nonce = %d, want 0 (reused, no gap)", n)
	}
	commit(true)
}

func TestNonce_MaxOfChainAndLocal(t *testing.T) {
	reader := &stubNonceReader{value: 5} // chain jumped ahead (e.g. an out-of-band tx)
	nm := chain.NewNonceManager(reader)
	ctx := context.Background()

	n, commit, _ := nm.Reserve(ctx, "0xa")
	if n != 5 {
		t.Fatalf("nonce = %d, want 5 (chain value)", n)
	}
	commit(true) // local next = 6

	reader.value = 0 // chain now lags the local counter
	n, commit, _ = nm.Reserve(ctx, "0xa")
	if n != 6 {
		t.Fatalf("nonce = %d, want 6 (max of chain=0, local=6)", n)
	}
	commit(true)
}

func TestNonce_ReserveError(t *testing.T) {
	nm := chain.NewNonceManager(&stubNonceReader{err: errors.New("rpc down")})
	_, commit, err := nm.Reserve(context.Background(), "0xa")
	if err == nil {
		t.Fatal("expected error from Reserve when GetNonce fails")
	}
	if commit != nil {
		t.Fatal("commit should be nil on error")
	}
}

func TestNonce_ConcurrentDistinct(t *testing.T) {
	nm := chain.NewNonceManager(&stubNonceReader{value: 0})
	const n = 50
	var wg sync.WaitGroup
	got := make([]uint64, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nonce, commit, err := nm.Reserve(context.Background(), "0xa")
			if err != nil {
				t.Errorf("Reserve: %v", err)
				return
			}
			got[i] = nonce
			commit(true)
		}(i)
	}
	wg.Wait()

	seen := map[uint64]bool{}
	for _, v := range got {
		if seen[v] {
			t.Fatalf("duplicate nonce %d assigned", v)
		}
		seen[v] = true
	}
	if len(seen) != n {
		t.Fatalf("distinct nonces = %d, want %d", len(seen), n)
	}
}
