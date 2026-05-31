package domain_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"

	"github.com/paylink/proof-validator/internal/chain"
	"github.com/paylink/proof-validator/internal/domain"
	"github.com/paylink/proof-validator/internal/httpx"
	"github.com/paylink/proof-validator/internal/proof"
	"github.com/paylink/proof-validator/internal/store/memory"
)

// ── test doubles ──

type fakeChain struct {
	used    bool
	usedErr error
	pl      *chain.PayLinkState
	plFound bool
	plErr   error
	sendErr error
	sent    []*lvm.Transaction
}

func (f *fakeChain) IsProofUsed(_ context.Context, _ string) (bool, error) { return f.used, f.usedErr }
func (f *fakeChain) GetPayLink(_ context.Context, _ string) (*chain.PayLinkState, bool, error) {
	return f.pl, f.plFound, f.plErr
}
func (f *fakeChain) SendTransaction(_ context.Context, tx *lvm.Transaction) (string, error) {
	if f.sendErr != nil {
		return "", f.sendErr
	}
	f.sent = append(f.sent, tx)
	return "0xtxhash", nil
}

type fakeVerifier struct{ err error }

func (f fakeVerifier) Verify(proof.Proof) error { return f.err }

type fakeSigner struct{ addr lvm.Address }

func (f fakeSigner) Address() lvm.Address { return f.addr }
func (f fakeSigner) SignTx(tx *lvm.Transaction) error {
	tx.Hash = lvm.SHA256Hash(tx.SignableBytes())
	tx.Signature = []byte{0x01}
	return nil
}

type fakeNonce struct {
	committed *bool
	err       error
}

func (f *fakeNonce) Reserve(_ context.Context, _ string) (uint64, func(bool), error) {
	if f.err != nil {
		return 0, nil, f.err
	}
	return 7, func(ok bool) {
		if f.committed != nil {
			*f.committed = ok
		}
	}, nil
}

type fakeMetrics struct {
	received []string
	settle   []string
}

func (f *fakeMetrics) ProofReceived(r string) { f.received = append(f.received, r) }
func (f *fakeMetrics) SettlementTx(r string)  { f.settle = append(f.settle, r) }

const signerAddr = "0x00000000000000000000000000000000000000aa"

func validProof() proof.Proof {
	return proof.Proof{
		PayLinkID: "0x" + strings.Repeat("ab", 32),
		Rail:      "mpesa",
		TxID:      "MP-1",
		Amount:    1500,
		Timestamp: 1730000000,
		Sender:    "254700000000",
		Receiver:  "254711111111",
		Signature: base64.StdEncoding.EncodeToString(make([]byte, 64)),
	}
}

func createdPayLink() *chain.PayLinkState {
	return &chain.PayLinkState{Status: "CREATED", Amount: 1500, Expiry: time.Now().Add(time.Hour).Unix()}
}

func newService(t *testing.T, store domain.Store, fc *fakeChain, v domain.ProofVerifier, m domain.ProofMetrics, opts ...domain.Option) *domain.Service {
	t.Helper()
	base := []domain.Option{}
	if m != nil {
		base = append(base, domain.WithMetrics(m))
	}
	base = append(base, opts...)
	return domain.NewService(store, fc, v, fakeSigner{addr: lvm.HexToAddress(signerAddr)}, &fakeNonce{}, nil, nil, base...)
}

// ── tests ──

func TestSubmitProof_Valid_Broadcasts(t *testing.T) {
	store := memory.New()
	fc := &fakeChain{pl: createdPayLink(), plFound: true}
	m := &fakeMetrics{}
	svc := newService(t, store, fc, fakeVerifier{}, m)

	p := validProof()
	res, err := svc.SubmitProof(context.Background(), p)
	if err != nil {
		t.Fatalf("SubmitProof: %v", err)
	}
	if res.Status != domain.StatusBroadcast || res.TxHash == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(fc.sent) != 1 {
		t.Fatalf("expected exactly 1 broadcast, got %d", len(fc.sent))
	}

	// The broadcast tx must be a TxSubmitValidation from the signer, carrying the canonical proofHash.
	tx := fc.sent[0]
	if tx.Type != lvm.TxSubmitValidation {
		t.Fatalf("tx type = %v, want TxSubmitValidation", tx.Type)
	}
	if tx.From != lvm.HexToAddress(signerAddr) {
		t.Fatalf("tx from = %s, want %s", tx.From.Hex(), signerAddr)
	}
	var payload lvm.SubmitValidationPayload
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	wantPH := lvm.ProofHash(lvm.HexToHash(p.PayLinkID), p.TxID, p.Amount)
	if payload.ProofHash != wantPH {
		t.Fatalf("proofHash = %s, want %s", payload.ProofHash.Hex(), wantPH.Hex())
	}
	if payload.PayLinkID != lvm.HexToHash(p.PayLinkID) {
		t.Fatalf("payload paylinkId mismatch")
	}

	// Persisted as broadcast with the tx hash.
	rec, err := store.GetByProofHash(context.Background(), wantPH.Hex())
	if err != nil {
		t.Fatalf("record not stored: %v", err)
	}
	if rec.Status != domain.StatusBroadcast || rec.TxHash != "0xtxhash" {
		t.Fatalf("stored record = %+v", rec)
	}
	if len(m.received) != 1 || m.received[0] != "accepted" {
		t.Fatalf("metrics.received = %v, want [accepted]", m.received)
	}
	if len(m.settle) != 1 || m.settle[0] != "broadcast" {
		t.Fatalf("metrics.settle = %v, want [broadcast]", m.settle)
	}
}

func TestSubmitProof_BadShape_NothingBroadcast(t *testing.T) {
	store := memory.New()
	fc := &fakeChain{pl: createdPayLink(), plFound: true}
	svc := newService(t, store, fc, fakeVerifier{}, nil)

	p := validProof()
	p.PayLinkID = "not-a-hash"
	_, err := svc.SubmitProof(context.Background(), p)
	assertCode(t, err, httpx.CodeInvalidProofShape)
	if len(fc.sent) != 0 {
		t.Fatalf("nothing should be broadcast on a bad shape, got %d", len(fc.sent))
	}
}

func TestSubmitProof_BadSignature_NothingBroadcast(t *testing.T) {
	store := memory.New()
	fc := &fakeChain{pl: createdPayLink(), plFound: true}
	v := fakeVerifier{err: httpx.NewError(httpx.CodeInvalidProofSignature, "bad", nil)}
	m := &fakeMetrics{}
	svc := newService(t, store, fc, v, m)

	_, err := svc.SubmitProof(context.Background(), validProof())
	assertCode(t, err, httpx.CodeInvalidProofSignature)
	if len(fc.sent) != 0 {
		t.Fatalf("nothing should be broadcast on a bad signature, got %d", len(fc.sent))
	}
	if len(m.received) != 1 || m.received[0] != "rejected_signature" {
		t.Fatalf("metrics.received = %v", m.received)
	}
}

func TestSubmitProof_AlreadySettled_NotRebroadcast(t *testing.T) {
	store := memory.New()
	fc := &fakeChain{used: true} // chain reports the proof already used (A.7)
	svc := newService(t, store, fc, fakeVerifier{}, nil)

	res, err := svc.SubmitProof(context.Background(), validProof())
	if err != nil {
		t.Fatalf("SubmitProof: %v", err)
	}
	if res.Status != domain.StatusAlreadySettled {
		t.Fatalf("status = %s, want already_settled", res.Status)
	}
	if len(fc.sent) != 0 {
		t.Fatalf("an already-settled proof must not be broadcast, got %d", len(fc.sent))
	}
}

func TestSubmitProof_CrossCheckRejections(t *testing.T) {
	cases := []struct {
		name  string
		pl    *chain.PayLinkState
		found bool
		want  httpx.ErrorCode
	}{
		{"not found", nil, false, httpx.CodePayLinkNotFound},
		{"not created", &chain.PayLinkState{Status: "VERIFIED", Amount: 1500, Expiry: time.Now().Add(time.Hour).Unix()}, true, httpx.CodePayLinkNotPayable},
		{"amount mismatch", &chain.PayLinkState{Status: "CREATED", Amount: 999, Expiry: time.Now().Add(time.Hour).Unix()}, true, httpx.CodeProofAmountMismatch},
		{"expired", &chain.PayLinkState{Status: "CREATED", Amount: 1500, Expiry: time.Now().Add(-time.Hour).Unix()}, true, httpx.CodePayLinkExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.New()
			fc := &fakeChain{pl: tc.pl, plFound: tc.found}
			svc := newService(t, store, fc, fakeVerifier{}, nil)
			_, err := svc.SubmitProof(context.Background(), validProof())
			assertCode(t, err, tc.want)
			if len(fc.sent) != 0 {
				t.Fatalf("nothing should be broadcast, got %d", len(fc.sent))
			}
		})
	}
}

func TestSubmitProof_CrossCheckDisabled_Broadcasts(t *testing.T) {
	store := memory.New()
	// No PayLink on chain, but cross-check is off → still broadcasts.
	fc := &fakeChain{plFound: false}
	svc := newService(t, store, fc, fakeVerifier{}, nil, domain.WithCrossCheck(false))

	res, err := svc.SubmitProof(context.Background(), validProof())
	if err != nil {
		t.Fatalf("SubmitProof: %v", err)
	}
	if res.Status != domain.StatusBroadcast || len(fc.sent) != 1 {
		t.Fatalf("expected a broadcast with cross-check off; res=%+v sent=%d", res, len(fc.sent))
	}
}

func TestSubmitProof_ChainUnavailableAtIsProofUsed(t *testing.T) {
	store := memory.New()
	fc := &fakeChain{usedErr: httpx.NewError(httpx.CodeChainUnavailable, "down", nil)}
	svc := newService(t, store, fc, fakeVerifier{}, nil)
	_, err := svc.SubmitProof(context.Background(), validProof())
	assertCode(t, err, httpx.CodeChainUnavailable)
	if len(fc.sent) != 0 {
		t.Fatalf("nothing should be broadcast")
	}
}

func TestSubmitProof_SendFails_RecordStaysReceived(t *testing.T) {
	store := memory.New()
	fc := &fakeChain{pl: createdPayLink(), plFound: true, sendErr: httpx.NewError(httpx.CodeChainUnavailable, "send failed", nil)}
	m := &fakeMetrics{}
	svc := newService(t, store, fc, fakeVerifier{}, m)

	_, err := svc.SubmitProof(context.Background(), validProof())
	assertCode(t, err, httpx.CodeChainUnavailable)

	ph := lvm.ProofHash(lvm.HexToHash(validProof().PayLinkID), validProof().TxID, validProof().Amount)
	rec, gerr := store.GetByProofHash(context.Background(), ph.Hex())
	if gerr != nil {
		t.Fatalf("record should exist (intent persisted before broadcast): %v", gerr)
	}
	if rec.Status != domain.StatusReceived {
		t.Fatalf("status = %s, want received (broadcast failed)", rec.Status)
	}
	if len(m.settle) != 1 || m.settle[0] != "error" {
		t.Fatalf("metrics.settle = %v, want [error]", m.settle)
	}
}

func TestSubmitProof_DuplicateReturnsExistingStatus(t *testing.T) {
	store := memory.New()
	p := validProof()
	ph := lvm.ProofHash(lvm.HexToHash(p.PayLinkID), p.TxID, p.Amount)
	// Pre-seed a prior broadcast record for the same proof_hash.
	_ = store.InsertProof(context.Background(), domain.ProofRecord{
		ProofHash: ph.Hex(), PayLinkID: p.PayLinkID, Rail: p.Rail, TxID: p.TxID, Amount: p.Amount,
		Status: domain.StatusBroadcast, TxHash: "0xprev", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	fc := &fakeChain{pl: createdPayLink(), plFound: true}
	svc := newService(t, store, fc, fakeVerifier{}, nil)

	res, err := svc.SubmitProof(context.Background(), p)
	if err != nil {
		t.Fatalf("SubmitProof: %v", err)
	}
	if res.Status != domain.StatusBroadcast || res.TxHash != "0xprev" {
		t.Fatalf("duplicate should return the existing record; got %+v", res)
	}
	if len(fc.sent) != 0 {
		t.Fatalf("a duplicate proof must not be re-broadcast, got %d", len(fc.sent))
	}
}

func TestGet_NotFound(t *testing.T) {
	svc := newService(t, memory.New(), &fakeChain{}, fakeVerifier{}, nil)
	_, err := svc.Get(context.Background(), "0xdoesnotexist")
	assertCode(t, err, httpx.CodeProofNotFound)
}

func TestGet_UpgradesToSettled(t *testing.T) {
	store := memory.New()
	ph := "0x" + strings.Repeat("cd", 32)
	_ = store.InsertProof(context.Background(), domain.ProofRecord{
		ProofHash: ph, PayLinkID: "0x" + strings.Repeat("ab", 32), Rail: "mpesa", TxID: "t", Amount: 1,
		Status: domain.StatusBroadcast, TxHash: "0xtx", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	fc := &fakeChain{used: true} // chain now reports the proof used
	svc := newService(t, store, fc, fakeVerifier{}, nil)

	rec, err := svc.Get(context.Background(), ph)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Status != domain.StatusSettled {
		t.Fatalf("status = %s, want settled", rec.Status)
	}
}

func assertCode(t *testing.T, err error, want httpx.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", want)
	}
	var ae *httpx.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not an AppError", err)
	}
	if ae.Code != want {
		t.Fatalf("error code = %s, want %s", ae.Code, want)
	}
}
