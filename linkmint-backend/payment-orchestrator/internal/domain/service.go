package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/paylink/payment-orchestrator/internal/httpx"
	"github.com/paylink/payment-orchestrator/internal/lifecycle"
)

// Clock and IDGen are injected for deterministic tests.
type Clock func() time.Time
type IDGen func() string

// Service coordinates the payment lifecycle. It is the conductor: it never holds funds (A.1),
// never decides settlement (A.3 — chain quorum does), and is idempotent on replays (A.7).
type Service struct {
	store     Store
	paylinks  PayLinkLookup
	chain     ChainReader
	publisher Publisher
	metrics   TransitionRecorder
	log       *slog.Logger
	now       Clock
	newID     IDGen
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the time source (tests).
func WithClock(c Clock) Option { return func(s *Service) { s.now = c } }

// WithIDGen overrides the id generator (tests).
func WithIDGen(g IDGen) Option { return func(s *Service) { s.newID = g } }

// WithMetrics attaches a transition recorder.
func WithMetrics(m TransitionRecorder) Option { return func(s *Service) { s.metrics = m } }

// NewService builds a Service. log may be nil (defaults to slog.Default).
func NewService(store Store, paylinks PayLinkLookup, chain ChainReader, publisher Publisher, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{
		store:     store,
		paylinks:  paylinks,
		chain:     chain,
		publisher: publisher,
		log:       log,
		now:       time.Now,
		newID:     uuid.NewString,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// InitiateInput is the validated input to Initiate.
type InitiateInput struct {
	PayLinkID string
	Rail      string
}

// payableStatuses are the live, unsettled paylink-service OffChainStatus values: CREATED
// (row written, on-chain submit in flight) and PENDING (submitted, awaiting validator
// quorum). With chain submit enabled a PayLink is PENDING by the time create returns, so
// PENDING must be payable (work35). Terminal states (VERIFIED/CANCELLED/FAILED/EXPIRED)
// are rejected.
var payableStatuses = map[string]bool{"CREATED": true, "PENDING": true}

// Initiate starts a payment lifecycle for an existing, payable PayLink. It validates the
// PayLink via paylink-service (the record owner), then records an AWAITING_PAYMENT payment.
// It moves no funds and verifies no proofs (that is work03/04).
func (s *Service) Initiate(ctx context.Context, in InitiateInput) (Payment, error) {
	plID := normalizeHash(in.PayLinkID)
	if !validHash(plID) {
		return Payment{}, httpx.NewError(httpx.CodeInvalidPayload, "paylink_id must be a 0x-prefixed 32-byte hex hash", nil)
	}
	if !ValidRail(in.Rail) {
		return Payment{}, httpx.NewError(httpx.CodeInvalidPayload, "rail must be one of mpesa|card|bank|crypto", map[string]any{"rail": in.Rail})
	}

	rec, err := s.paylinks.GetPayLink(ctx, plID)
	if err != nil {
		return Payment{}, err // PAYLINK_SERVICE_UNAVAILABLE from the client
	}
	if rec == nil {
		return Payment{}, httpx.NewError(httpx.CodePayLinkNotFound, "no PayLink with that id", map[string]any{"paylink_id": plID})
	}
	if !payableStatuses[rec.Status] {
		return Payment{}, httpx.NewError(httpx.CodePayLinkNotPayable,
			fmt.Sprintf("PayLink is %s and cannot accept a new payment", rec.Status),
			map[string]any{"paylink_id": plID, "status": rec.Status})
	}
	if !rec.Expiry.IsZero() && s.now().After(rec.Expiry) {
		return Payment{}, httpx.NewError(httpx.CodePayLinkExpired, "PayLink has expired", map[string]any{"paylink_id": plID, "expiry": rec.Expiry.UTC().Format(time.RFC3339)})
	}

	now := s.now().UTC()
	p := Payment{
		ID:        s.newID(),
		PayLinkID: plID,
		Rail:      in.Rail,
		Status:    lifecycle.StateAwaitingPayment,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreatePayment(ctx, p); err != nil {
		if errors.Is(err, ErrPaymentExists) {
			existing, getErr := s.store.GetPaymentByPayLink(ctx, plID)
			details := map[string]any{"paylink_id": plID}
			if getErr == nil {
				details["payment_id"] = existing.ID
				details["status"] = string(existing.Status)
			}
			return Payment{}, httpx.NewError(httpx.CodePaymentExists, "a payment already exists for this PayLink", details)
		}
		return Payment{}, err
	}

	s.publish(ctx, EventPaymentInitiated, p)
	s.log.Info("payment_initiated", "payment_id", p.ID, "paylink_id", plID, "rail", in.Rail)
	return p, nil
}

// Get returns a payment, reconciled against on-chain truth so the response is consistent with
// chain state (A.3). A transient chain error degrades gracefully: the stored record is returned.
func (s *Service) Get(ctx context.Context, id string) (Payment, error) {
	p, err := s.store.GetPayment(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Payment{}, httpx.NewError(httpx.CodePaymentNotFound, "no payment with that id", map[string]any{"payment_id": id})
		}
		return Payment{}, err
	}
	if updated, changed := s.reconcile(ctx, p); changed {
		return updated, nil
	}
	return p, nil
}

// Search returns payments matching q (an exact id/paylink_id or a status). It is the read-only
// admin lookup (admin-backoffice, work11): no chain reconcile, no funds, no settlement decision.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]Payment, error) {
	return s.store.SearchPayments(ctx, q, limit)
}

// reconcile reads on-chain status and advances the stored payment toward it. It is the read-path
// safety net that also closes any gap from missed WS events. Idempotent and best-effort.
func (s *Service) reconcile(ctx context.Context, p Payment) (Payment, bool) {
	if lifecycle.IsTerminal(p.Status) {
		return p, false
	}
	status, found, err := s.chain.PayLinkStatus(ctx, p.PayLinkID)
	if err != nil {
		s.log.Warn("reconcile_chain_unavailable", "paylink_id", p.PayLinkID, "err", err.Error())
		return p, false
	}
	if !found {
		return p, false
	}
	updated, changed, err := s.store.Reconcile(ctx, p.PayLinkID, projectTo(status))
	if err != nil {
		if !errors.Is(err, lifecycle.ErrIllegalTransition) && !errors.Is(err, ErrNotFound) {
			s.log.Warn("reconcile_failed", "paylink_id", p.PayLinkID, "err", err.Error())
		}
		return p, false
	}
	if changed {
		s.afterTransition(ctx, updated)
	}
	return updated, changed
}

// ChainEventInput is a normalized chain event handed to the service by the subscriber.
type ChainEventInput struct {
	PayLinkID   string
	Seq         uint64
	ChainStatus string // authoritative on-chain status: VERIFIED|CANCELLED|FAILED|CREATED
	Kind        string // chain event kind, for audit (e.g. "paylink.verified")
	TxHash      string
}

// ApplyChainEvent advances the payment for the event's PayLink toward on-chain truth, atomically
// and idempotently. Duplicate events (same PayLinkID+Seq) and replays of an already-applied
// status are no-ops (changed=false) — this is the A.7 anti-replay guarantee at the lifecycle
// layer. Events for PayLinks this service is not orchestrating return ErrNotFound (ignored).
func (s *Service) ApplyChainEvent(ctx context.Context, in ChainEventInput) (Payment, bool, error) {
	plID := normalizeHash(in.PayLinkID)
	ref := ChainEventRef{PayLinkID: plID, Seq: in.Seq, Kind: in.Kind, TxHash: in.TxHash}
	updated, changed, err := s.store.ApplyChainEvent(ctx, ref, projectTo(in.ChainStatus))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return Payment{}, false, ErrNotFound
		case errors.Is(err, lifecycle.ErrIllegalTransition):
			s.log.Warn("chain_event_illegal_transition", "paylink_id", plID, "chain_status", in.ChainStatus, "kind", in.Kind)
			return updated, false, nil
		default:
			return Payment{}, false, err
		}
	}
	if changed {
		s.afterTransition(ctx, updated)
		s.log.Info("payment_advanced", "payment_id", updated.ID, "paylink_id", plID, "status", string(updated.Status), "kind", in.Kind, "seq", in.Seq)
	}
	return updated, changed, nil
}

// projectTo returns a ProjectFn that projects onto the given on-chain status.
func projectTo(chainStatus string) ProjectFn {
	return func(cur lifecycle.State) (lifecycle.State, bool, error) {
		return lifecycle.Project(cur, chainStatus)
	}
}

// afterTransition records metrics and publishes the matching domain event. The only non-terminal
// predecessor in this FSM is AWAITING_PAYMENT, so that is the recorded "from".
func (s *Service) afterTransition(ctx context.Context, updated Payment) {
	if s.metrics != nil {
		s.metrics.Transition(string(lifecycle.StateAwaitingPayment), string(updated.Status))
	}
	if name := domainEventForState(updated.Status); name != "" {
		s.publish(ctx, name, updated)
	}
}

func (s *Service) publish(ctx context.Context, name string, p Payment) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.Publish(ctx, name, p.PayLinkID, p.payload()); err != nil {
		s.log.Warn("event_publish_failed", "event", name, "payment_id", p.ID, "err", err.Error())
	}
}

// Ready reports whether the service's hard dependencies (the store) are reachable.
func (s *Service) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}
