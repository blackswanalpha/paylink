package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"

	"github.com/paylink/proof-validator/internal/events"
	"github.com/paylink/proof-validator/internal/httpx"
	"github.com/paylink/proof-validator/internal/proof"
)

// Proof record statuses.
const (
	StatusReceived       = "received"
	StatusBroadcast      = "broadcast"
	StatusAlreadySettled = "already_settled"
	StatusSettled        = "settled"
)

// Service is the proof-validator spine: verify a proof, then build/sign/broadcast a settlement
// transaction. It is non-custodial (A.1), defers settlement finality to the chain's quorum (A.3),
// only accepts the normalized proof shape (A.4), and never re-broadcasts a settled proof (A.7).
type Service struct {
	store      Store
	chain      ChainClient
	verifier   ProofVerifier
	signer     Signer
	nonce      NonceReserver
	publisher  Publisher
	metrics    ProofMetrics
	log        *slog.Logger
	crossCheck bool
	now        func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithMetrics attaches a metrics recorder.
func WithMetrics(m ProofMetrics) Option { return func(s *Service) { s.metrics = m } }

// WithClock overrides the time source (tests).
func WithClock(c func() time.Time) Option { return func(s *Service) { s.now = c } }

// WithCrossCheck toggles the on-chain PayLink cross-check (default true).
func WithCrossCheck(b bool) Option { return func(s *Service) { s.crossCheck = b } }

// NewService builds a Service. log may be nil (defaults to slog.Default).
func NewService(store Store, chain ChainClient, verifier ProofVerifier, signer Signer, nonce NonceReserver, publisher Publisher, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{
		store:      store,
		chain:      chain,
		verifier:   verifier,
		signer:     signer,
		nonce:      nonce,
		publisher:  publisher,
		log:        log,
		crossCheck: true,
		now:        time.Now,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Result is the outcome of SubmitProof.
type Result struct {
	ProofHash string
	TxHash    string
	Status    string
}

// SubmitProof verifies a proof and broadcasts a settlement transaction for its PayLink. Order:
// shape → signature → A.7 pre-check → cross-check → persist → build/sign/broadcast → mark. On any
// rejection before broadcast, nothing is sent to the chain.
func (s *Service) SubmitProof(ctx context.Context, p proof.Proof) (res Result, err error) {
	// recordOutcome handles only rejections; success outcomes are labeled explicitly below so a
	// duplicate replay is not miscounted as a fresh acceptance.
	defer func() { s.recordOutcome(err) }()

	if err = proof.ValidateShape(p); err != nil {
		return Result{}, err
	}
	if err = s.verifier.Verify(p); err != nil {
		return Result{}, err
	}

	plID := lvm.HexToHash(p.PayLinkID)
	proofHash := lvm.ProofHash(plID, p.TxID, p.Amount)
	phHex := proofHash.Hex()

	// A.7: the chain is the source of truth. If the proof already settled, never broadcast again.
	used, ierr := s.chain.IsProofUsed(ctx, phHex)
	if ierr != nil {
		return Result{}, ierr
	}
	if used {
		s.recordAlreadySettled(ctx, p, phHex)
		s.recordReceived("already_settled")
		return Result{ProofHash: phHex, Status: StatusAlreadySettled}, nil
	}

	// The chain settles on status/expiry/proof-usage but does NOT check the amount — so the
	// validator enforces amount integrity here (the off-chain enforcement point the chain lacks).
	if s.crossCheck {
		if cerr := s.crossCheckPayLink(ctx, p); cerr != nil {
			return Result{}, cerr
		}
	}

	// Persist intent first: the unique proof_hash is the local anti-double-broadcast guard that
	// complements on-chain A.7. A concurrent duplicate races on the DB index, not on the chain.
	now := s.now().UTC()
	if ierr := s.store.InsertProof(ctx, ProofRecord{
		ProofHash: phHex, PayLinkID: p.PayLinkID, Rail: p.Rail, TxID: p.TxID, Amount: p.Amount,
		Status: StatusReceived, CreatedAt: now, UpdatedAt: now,
	}); ierr != nil {
		if errors.Is(ierr, ErrProofExists) {
			dup, derr := s.resultForExisting(ctx, phHex)
			if derr == nil {
				s.recordReceived("duplicate")
			}
			return dup, derr
		}
		return Result{}, ierr
	}
	s.publish(ctx, events.ProofReceived, phHex, map[string]any{"paylink_id": p.PayLinkID, "rail": p.Rail})

	txHash, berr := s.broadcastSettlement(ctx, plID, proofHash)
	if berr != nil {
		// tx not broadcast; the record stays `received` for a later retry.
		return Result{}, berr
	}

	if merr := s.store.MarkBroadcast(ctx, phHex, txHash, StatusBroadcast); merr != nil {
		// The tx is already in the mempool — don't fail the response over a bookkeeping write.
		s.log.Warn("mark_broadcast_failed", "proof_hash", phHex, "err", merr.Error())
	}
	s.publish(ctx, events.ProofSettlementBroadcast, phHex, map[string]any{"tx_hash": txHash, "paylink_id": p.PayLinkID})
	s.log.Info("settlement_tx_broadcast", "proof_hash", phHex, "tx_hash", txHash, "paylink_id", p.PayLinkID)
	s.recordReceived("accepted")
	return Result{ProofHash: phHex, TxHash: txHash, Status: StatusBroadcast}, nil
}

// broadcastSettlement reserves a nonce, builds+signs the TxSubmitValidation, and broadcasts it.
// The nonce reservation is committed only on a successful send (a failed send leaves no gap).
func (s *Service) broadcastSettlement(ctx context.Context, plID, proofHash lvm.Hash) (string, error) {
	from := s.signer.Address()
	nonce, commit, err := s.nonce.Reserve(ctx, from.Hex())
	if err != nil {
		return "", err // CHAIN_UNAVAILABLE from the nonce read
	}
	// Panic safety: commit is sync.Once-guarded, so this deferred call no-ops once the explicit
	// commit(ok) below has run, but it releases the nonce lock if a panic occurs in between.
	defer commit(false)

	tx, err := lvm.BuildSubmitValidationTx(from, nonce, plID, proofHash)
	if err != nil {
		commit(false)
		return "", httpx.NewError(httpx.CodeInternalError, "build settlement tx: "+err.Error(), nil)
	}
	if err := s.signer.SignTx(tx); err != nil {
		commit(false)
		return "", httpx.NewError(httpx.CodeInternalError, "sign settlement tx: "+err.Error(), nil)
	}

	txHash, sendErr := s.chain.SendTransaction(ctx, tx)
	commit(sendErr == nil)
	if s.metrics != nil {
		if sendErr != nil {
			s.metrics.SettlementTx("error")
		} else {
			s.metrics.SettlementTx("broadcast")
		}
	}
	if sendErr != nil {
		return "", sendErr // CHAIN_UNAVAILABLE
	}
	return txHash, nil
}

// crossCheckPayLink rejects proofs that cannot legitimately settle: unknown / non-CREATED /
// amount-mismatched / expired PayLinks. Transport failures surface as CHAIN_UNAVAILABLE.
func (s *Service) crossCheckPayLink(ctx context.Context, p proof.Proof) error {
	pl, found, err := s.chain.GetPayLink(ctx, p.PayLinkID)
	if err != nil {
		return err
	}
	if !found {
		return httpx.NewError(httpx.CodePayLinkNotFound, "no PayLink with that id", map[string]any{"paylink_id": p.PayLinkID})
	}
	if pl.Status != "CREATED" {
		return httpx.NewError(httpx.CodePayLinkNotPayable,
			fmt.Sprintf("PayLink is %s and cannot be settled", pl.Status),
			map[string]any{"paylink_id": p.PayLinkID, "status": pl.Status})
	}
	if pl.Amount != p.Amount {
		return httpx.NewError(httpx.CodeProofAmountMismatch, "proof amount does not match the PayLink amount",
			map[string]any{"proof_amount": p.Amount, "paylink_amount": pl.Amount})
	}
	if pl.Expiry != 0 && s.now().Unix() > pl.Expiry {
		return httpx.NewError(httpx.CodePayLinkExpired, "PayLink has expired",
			map[string]any{"paylink_id": p.PayLinkID, "expiry": pl.Expiry})
	}
	// NOTE: receiver/sender binding is intentionally NOT cross-checked here. The proof's
	// sender/receiver are rail-level identifiers (e.g. MSISDNs) while the PayLink receiver is an
	// on-chain address — different namespaces with no MVP mapping. The receiver is still
	// authenticated as part of the adapter-signed proof; binding it to the on-chain receiver is
	// deferred to a future identity-mapping work item.
	return nil
}

// recordAlreadySettled best-effort records a proof the chain already settled (audit trail).
func (s *Service) recordAlreadySettled(ctx context.Context, p proof.Proof, phHex string) {
	now := s.now().UTC()
	err := s.store.InsertProof(ctx, ProofRecord{
		ProofHash: phHex, PayLinkID: p.PayLinkID, Rail: p.Rail, TxID: p.TxID, Amount: p.Amount,
		Status: StatusAlreadySettled, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil && !errors.Is(err, ErrProofExists) {
		s.log.Warn("record_already_settled_failed", "proof_hash", phHex, "err", err.Error())
	}
}

// resultForExisting returns the current status for a proof_hash already recorded locally, so a
// duplicate submission is idempotent (no second broadcast). A read failure here is an internal
// error (the row collided on insert but cannot be read back) — not a benign conflict — so we
// surface it rather than masking it as PROOF_EXISTS.
func (s *Service) resultForExisting(ctx context.Context, phHex string) (Result, error) {
	rec, err := s.store.GetByProofHash(ctx, phHex)
	if err != nil {
		return Result{}, httpx.NewError(httpx.CodeInternalError,
			"proof already recorded but could not be read: "+err.Error(), map[string]any{"proof_hash": phHex})
	}
	return Result{ProofHash: phHex, TxHash: rec.TxHash, Status: rec.Status}, nil
}

// Get returns the stored proof record, upgrading status to "settled" if the chain now reports the
// proof used. Returns PROOF_NOT_FOUND when unknown.
func (s *Service) Get(ctx context.Context, proofHash string) (ProofRecord, error) {
	rec, err := s.store.GetByProofHash(ctx, proofHash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ProofRecord{}, httpx.NewError(httpx.CodeProofNotFound, "no proof with that hash", map[string]any{"proof_hash": proofHash})
		}
		return ProofRecord{}, err
	}
	// A broadcast — or a record left `received` after a post-send bookkeeping write failed —
	// settles once the chain reports the proof used; reconcile on read so status converges to truth.
	if rec.Status == StatusBroadcast || rec.Status == StatusReceived {
		if used, uerr := s.chain.IsProofUsed(ctx, proofHash); uerr == nil && used {
			rec.Status = StatusSettled
		}
	}
	return rec, nil
}

// Ready reports whether the service's store dependency is reachable.
func (s *Service) Ready(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) publish(ctx context.Context, name, key string, payload any) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.Publish(ctx, name, key, payload); err != nil {
		s.log.Warn("event_publish_failed", "event", name, "err", err.Error())
	}
}

// recordReceived records a proofs_received_total outcome (nil-safe).
func (s *Service) recordReceived(label string) {
	if s.metrics != nil {
		s.metrics.ProofReceived(label)
	}
}

// recordOutcome records the proofs_received_total metric for a REJECTED request. Success outcomes
// (accepted / already_settled / duplicate) are recorded explicitly at their call sites so a
// duplicate replay is never miscounted as a fresh acceptance.
func (s *Service) recordOutcome(err error) {
	if err != nil {
		s.recordReceived(resultLabelForError(err))
	}
}

func resultLabelForError(err error) string {
	ae := httpx.AsAppError(err)
	switch ae.Code {
	case httpx.CodeInvalidProofShape, httpx.CodeInvalidPayload:
		return "rejected_shape"
	case httpx.CodeInvalidProofSignature:
		return "rejected_signature"
	case httpx.CodeChainUnavailable:
		return "chain_unavailable"
	case httpx.CodePayLinkNotFound, httpx.CodePayLinkNotPayable, httpx.CodePayLinkExpired, httpx.CodeProofAmountMismatch, httpx.CodeProofExists:
		return "rejected_paylink"
	default:
		return "error"
	}
}
