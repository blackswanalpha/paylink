package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/paylink/escrow-manager/internal/fsm"
	"github.com/paylink/escrow-manager/internal/httpx"
)

// Clock and IDGen are injected for deterministic tests.
type Clock func() time.Time
type IDGen func() string

// Consumer-handle results (escrow_events_consumed_total{result}).
const (
	ResultFunded    = "funded"    // funded flag set; condition not yet satisfied
	ResultReleased  = "released"  // funded + condition satisfied → RELEASED in one tx
	ResultDuplicate = "duplicate" // (pl_id, tx_hash) already processed (DbDedupe)
	ResultSkipped   = "skipped"   // escrow no longer WAITING (late event; DISPUTED blocks it)
	ResultIgnored   = "ignored"   // no escrow for the PayLink — not ours
)

// Service coordinates the escrow lifecycle. It never holds funds (A.1 — release/refund are
// instructions), never decides funding (A.3 — the chain.paylink.verified event does), and is
// idempotent on replays (A.7 — DbDedupe + approval PK + CAS state updates).
type Service struct {
	store          Store
	publisher      Publisher
	metrics        TransitionRecorder
	log            *slog.Logger
	machine        *fsm.Machine
	now            Clock
	newID          IDGen
	defaultTimeout time.Duration
}

// Option configures a Service.
type Option func(*Service)

// WithClock overrides the time source (tests).
func WithClock(c Clock) Option { return func(s *Service) { s.now = c } }

// WithIDGen overrides the id generator (tests).
func WithIDGen(g IDGen) Option { return func(s *Service) { s.newID = g } }

// WithMetrics attaches a transition recorder.
func WithMetrics(m TransitionRecorder) Option { return func(s *Service) { s.metrics = m } }

// WithDefaultTimeout overrides the timeout_at default applied on create.
func WithDefaultTimeout(d time.Duration) Option { return func(s *Service) { s.defaultTimeout = d } }

// NewService builds a Service. log may be nil (defaults to slog.Default).
func NewService(store Store, publisher Publisher, log *slog.Logger, opts ...Option) *Service {
	if log == nil {
		log = slog.Default()
	}
	s := &Service{
		store:          store,
		publisher:      publisher,
		log:            log,
		machine:        fsm.NewEscrowMachine(),
		now:            time.Now,
		newID:          newEscrowID,
		defaultTimeout: 24 * time.Hour,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// newEscrowID generates an ESC_-prefixed identifier.
func newEscrowID() string {
	return "ESC_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// CreateInput is the validated input to Create.
type CreateInput struct {
	CreatorAddr     string
	PLID            string
	PayeeAddr       string
	RefundTo        string
	Amount          string
	Currency        string
	ConditionType   string
	ConditionParams ConditionParams
	TimeoutAt       *time.Time
}

// Create records a new WAITING escrow for a PayLink. It validates the condition per type and
// publishes escrow.created. It moves no funds and verifies no settlement (A.1/A.3).
func (s *Service) Create(ctx context.Context, in CreateInput) (Escrow, error) {
	creator := normalizeAddr(in.CreatorAddr)
	plID := strings.TrimSpace(in.PLID)
	payee := normalizeAddr(in.PayeeAddr)
	refundTo := normalizeAddr(in.RefundTo)

	if plID == "" || len(plID) > 128 {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, "pl_id is required (opaque PayLink reference, ≤128 chars)", nil)
	}
	if payee == "" {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, "payee_addr is required", nil)
	}
	if refundTo == "" {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, "refund_to is required", nil)
	}
	amount, err := normalizeAmount(in.Amount)
	if err != nil {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, err.Error(), map[string]any{"amount": in.Amount})
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" || len(currency) > 16 {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, "currency is required (≤16 chars)", nil)
	}
	if !ValidConditionType(in.ConditionType) {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload,
			"condition_type must be one of delivery_confirmation|time_lock|multi_party_approval",
			map[string]any{"condition_type": in.ConditionType})
	}

	now := s.now().UTC()
	timeoutAt := now.Add(s.defaultTimeout)
	if in.TimeoutAt != nil {
		if !in.TimeoutAt.After(now) {
			return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, "timeout_at must be in the future", nil)
		}
		timeoutAt = in.TimeoutAt.UTC()
	}
	params, releaseAt, err := validateCondition(in.ConditionType, in.ConditionParams, now, timeoutAt)
	if err != nil {
		return Escrow{}, err
	}

	e := Escrow{
		ID:              s.newID(),
		PLID:            plID,
		CreatorAddr:     creator,
		PayeeAddr:       payee,
		RefundTo:        refundTo,
		Amount:          amount,
		Currency:        currency,
		ConditionType:   in.ConditionType,
		ConditionParams: params,
		State:           fsm.StateWaiting,
		ReleaseAt:       releaseAt,
		TimeoutAt:       timeoutAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.CreateEscrow(ctx, e); err != nil {
		if errors.Is(err, ErrEscrowExists) {
			return Escrow{}, httpx.NewError(httpx.CodeEscrowExists,
				"an escrow already exists for this PayLink", map[string]any{"pl_id": plID})
		}
		return Escrow{}, err
	}

	s.publish(ctx, EventEscrowCreated, e, e.createdPayload())
	s.log.Info("escrow_created", "escrow_id", e.ID, "pl_id", plID, "condition_type", e.ConditionType)
	return e, nil
}

// validateCondition checks per-type condition params and returns the canonical params plus the
// release_at column value (time_lock only).
func validateCondition(condType string, p ConditionParams, now, timeoutAt time.Time) (ConditionParams, *time.Time, error) {
	switch condType {
	case ConditionDeliveryConfirmation:
		if p.ReleaseAt != nil || len(p.Approvers) > 0 || p.Threshold != 0 {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"delivery_confirmation takes no condition_params", nil)
		}
		return ConditionParams{}, nil, nil

	case ConditionTimeLock:
		if len(p.Approvers) > 0 || p.Threshold != 0 {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"time_lock takes only condition_params.release_at", nil)
		}
		if p.ReleaseAt == nil {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"time_lock requires condition_params.release_at", nil)
		}
		releaseAt := p.ReleaseAt.UTC()
		if !releaseAt.After(now) {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"condition_params.release_at must be in the future", nil)
		}
		if !releaseAt.Before(timeoutAt) {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"condition_params.release_at must be before timeout_at", nil)
		}
		return ConditionParams{ReleaseAt: &releaseAt}, &releaseAt, nil

	case ConditionMultiPartyApproval:
		if p.ReleaseAt != nil {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"multi_party_approval takes only condition_params.approvers and .threshold", nil)
		}
		if len(p.Approvers) == 0 {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				"multi_party_approval requires a non-empty condition_params.approvers list", nil)
		}
		seen := map[string]bool{}
		approvers := make([]string, 0, len(p.Approvers))
		for _, a := range p.Approvers {
			a = normalizeAddr(a)
			if a == "" {
				return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
					"condition_params.approvers must not contain empty addresses", nil)
			}
			if seen[a] {
				return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
					"condition_params.approvers must be unique", map[string]any{"approver": a})
			}
			seen[a] = true
			approvers = append(approvers, a)
		}
		if p.Threshold < 1 || p.Threshold > len(approvers) {
			return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload,
				fmt.Sprintf("condition_params.threshold must be between 1 and %d", len(approvers)),
				map[string]any{"threshold": p.Threshold})
		}
		return ConditionParams{Approvers: approvers, Threshold: p.Threshold}, nil, nil
	}
	return ConditionParams{}, nil, httpx.NewError(httpx.CodeInvalidPayload, "unknown condition_type", nil)
}

// Get returns an escrow by id, scoped to callers allowed to view it (participants +
// multi-party approvers). Outsiders get the same 404 as a missing id so escrow ids
// don't leak existence.
func (s *Service) Get(ctx context.Context, id, caller string) (Escrow, error) {
	e, err := s.store.GetEscrow(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Escrow{}, httpx.NewError(httpx.CodeEscrowNotFound, "no escrow with that id", map[string]any{"escrow_id": id})
		}
		return Escrow{}, err
	}
	if !e.CanView(normalizeAddr(caller)) {
		return Escrow{}, httpx.NewError(httpx.CodeEscrowNotFound, "no escrow with that id", map[string]any{"escrow_id": id})
	}
	return e, nil
}

// List returns the caller's escrows, optionally filtered by state.
func (s *Service) List(ctx context.Context, creatorAddr, state string, limit int) ([]Escrow, error) {
	state = strings.ToUpper(strings.TrimSpace(state))
	if state != "" && !fsm.ValidState(fsm.State(state)) {
		return nil, httpx.NewError(httpx.CodeInvalidPayload,
			"state must be one of WAITING|CONDITIONS_MET|RELEASED|REFUNDED|DISPUTED",
			map[string]any{"state": state})
	}
	return s.store.ListEscrows(ctx, normalizeAddr(creatorAddr), state, limit)
}

// Confirm records the caller's confirmation/approval and — when the condition is satisfied AND
// the escrow is funded — applies ConditionsMet+Release together in one store transaction.
//   - delivery_confirmation: creator only.
//   - multi_party_approval: caller must be in the approvers list; approvals are idempotent (PK).
//   - time_lock: not confirmable (releases automatically at release_at).
func (s *Service) Confirm(ctx context.Context, id, callerAddr string) (Escrow, error) {
	caller := normalizeAddr(callerAddr)
	var kinds []string
	e, err := s.store.Mutate(ctx, id, func(cur Escrow, approvals []string) (Update, error) {
		switch cur.ConditionType {
		case ConditionTimeLock:
			return Update{}, httpx.NewError(httpx.CodeConditionNotConfirmable,
				"a time_lock escrow releases automatically at release_at and cannot be confirmed",
				map[string]any{"escrow_id": cur.ID})
		case ConditionDeliveryConfirmation:
			if caller != cur.CreatorAddr {
				return Update{}, httpx.NewError(httpx.CodeNotParticipant,
					"only the escrow creator can confirm delivery", map[string]any{"escrow_id": cur.ID})
			}
		case ConditionMultiPartyApproval:
			if !contains(cur.ConditionParams.Approvers, caller) {
				return Update{}, httpx.NewError(httpx.CodeNotParticipant,
					"caller is not in the approvers list", map[string]any{"escrow_id": cur.ID})
			}
		}
		if cur.State != fsm.StateWaiting {
			return Update{}, httpx.NewError(httpx.CodeInvalidState,
				fmt.Sprintf("escrow is %s and cannot be confirmed", cur.State),
				map[string]any{"escrow_id": cur.ID, "state": string(cur.State)})
		}

		up := Update{AddApproval: caller}
		satisfied := conditionSatisfied(cur, appendApproval(approvals, caller), s.now().UTC())
		if next, k, ok := s.tryRelease(cur.State, cur.Funded, satisfied); ok {
			up.SetState = next
			kinds = k
		}
		return up, nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Escrow{}, httpx.NewError(httpx.CodeEscrowNotFound, "no escrow with that id", map[string]any{"escrow_id": id})
		}
		return Escrow{}, err
	}
	if len(kinds) > 0 {
		s.recordTransitions(kinds)
		s.publish(ctx, EventEscrowReleased, e, e.releasedPayload())
		s.log.Info("escrow_released", "escrow_id", e.ID, "pl_id", e.PLID, "via", "confirm")
	}
	return e, nil
}

// Dispute moves a WAITING (or CONDITIONS_MET) escrow to DISPUTED. Participants only. DISPUTED
// is terminal here (resolution is work22): it blocks the sweeper and the funding consumer.
func (s *Service) Dispute(ctx context.Context, id, callerAddr, reason string) (Escrow, error) {
	caller := normalizeAddr(callerAddr)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Escrow{}, httpx.NewError(httpx.CodeInvalidPayload, "reason is required", nil)
	}
	disputed := false
	e, err := s.store.Mutate(ctx, id, func(cur Escrow, _ []string) (Update, error) {
		if !cur.IsParticipant(caller) {
			return Update{}, httpx.NewError(httpx.CodeNotParticipant,
				"only the creator, payee, or refund recipient can dispute", map[string]any{"escrow_id": cur.ID})
		}
		next, err := s.machine.Apply(cur.State, fsm.KindDispute, nil)
		if err != nil {
			return Update{}, httpx.NewError(httpx.CodeInvalidState,
				fmt.Sprintf("escrow is %s and cannot be disputed", cur.State),
				map[string]any{"escrow_id": cur.ID, "state": string(cur.State)})
		}
		disputed = true
		return Update{SetState: next, DisputeReason: reason}, nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Escrow{}, httpx.NewError(httpx.CodeEscrowNotFound, "no escrow with that id", map[string]any{"escrow_id": id})
		}
		return Escrow{}, err
	}
	if disputed {
		s.recordTransitions([]string{"dispute"})
		s.publish(ctx, EventEscrowDisputed, e, e.disputedPayload())
		s.log.Info("escrow_disputed", "escrow_id", e.ID, "pl_id", e.PLID)
	}
	return e, nil
}

// HandlePaylinkVerified applies funding truth from a chain.paylink.verified event: it sets the
// funded flag (and tx hash) when the escrow is still WAITING, evaluates the condition, and —
// when satisfied — releases in the same transaction. The store dedupes on (pl_id, tx_hash) via
// a processed_events row in that transaction, so redeliveries are exactly-once in effect.
// The returned result feeds escrow_events_consumed_total{result}.
func (s *Service) HandlePaylinkVerified(ctx context.Context, plID, txHash string) (string, error) {
	plID = strings.TrimSpace(plID)
	var fundedNow bool
	var kinds []string
	e, applied, err := s.store.ApplyFunding(ctx, plID, txHash, func(cur Escrow, approvals []string) (Update, error) {
		if cur.State != fsm.StateWaiting {
			// Late funding event (already released/refunded, or DISPUTED — which blocks the
			// consumer). Record the dedupe row, change nothing.
			return Update{}, nil
		}
		up := Update{}
		if !cur.Funded {
			up.SetFunded = true
			up.FundedTxHash = txHash
			fundedNow = true
		}
		satisfied := conditionSatisfied(cur, approvals, s.now().UTC())
		if next, k, ok := s.tryRelease(cur.State, true, satisfied); ok {
			up.SetState = next
			kinds = k
		}
		return up, nil
	})
	if errors.Is(err, ErrNotFound) {
		return ResultIgnored, nil // the PayLink is not under escrow — not ours
	}
	if err != nil {
		return "", err
	}
	if !applied {
		return ResultDuplicate, nil
	}
	if len(kinds) > 0 {
		s.recordTransitions(kinds)
		s.publish(ctx, EventEscrowReleased, e, e.releasedPayload())
		s.log.Info("escrow_released", "escrow_id", e.ID, "pl_id", e.PLID, "via", "funding")
		return ResultReleased, nil
	}
	if fundedNow {
		s.log.Info("escrow_funded", "escrow_id", e.ID, "pl_id", e.PLID, "tx_hash", txHash)
		return ResultFunded, nil
	}
	return ResultSkipped, nil
}

// Sweep runs one sweeper pass: (1) release due funded time_locks, then (2) refund timeouts.
// Both are CAS updates (state='WAITING' in the WHERE clause), so DISPUTED and already-advanced
// rows are never touched. Errors are logged — the sweeper loop never dies.
func (s *Service) Sweep(ctx context.Context) {
	now := s.now().UTC()

	released, err := s.store.ReleaseDueTimeLocks(ctx, now)
	if err != nil {
		s.log.Error("sweep_release_failed", "err", err.Error())
	}
	for _, e := range released {
		s.recordTransitions([]string{"conditions_met", "release"})
		s.publish(ctx, EventEscrowReleased, e, e.releasedPayload())
		s.log.Info("escrow_released", "escrow_id", e.ID, "pl_id", e.PLID, "via", "time_lock")
	}

	refunded, err := s.store.RefundTimedOut(ctx, now)
	if err != nil {
		s.log.Error("sweep_timeout_failed", "err", err.Error())
	}
	for _, e := range refunded {
		s.recordTransitions([]string{"timeout"})
		s.publish(ctx, EventEscrowRefunded, e, e.refundedPayload())
		s.log.Info("escrow_refunded", "escrow_id", e.ID, "pl_id", e.PLID, "funded", e.Funded)
	}
}

// Ready reports whether the service's hard dependency (the store) is reachable.
func (s *Service) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}

// tryRelease applies ConditionsMet then Release on the machine. ok=false when the guard
// rejects (not funded / not satisfied) or the state has no such transition.
func (s *Service) tryRelease(cur fsm.State, funded, satisfied bool) (fsm.State, []string, bool) {
	mid, err := s.machine.Apply(cur, fsm.KindConditionsMet, fsm.ConditionsMetInput{Funded: funded, Satisfied: satisfied})
	if err != nil {
		return cur, nil, false
	}
	final, err := s.machine.Apply(mid, fsm.KindRelease, nil)
	if err != nil {
		return cur, nil, false
	}
	return final, []string{"conditions_met", "release"}, true
}

// conditionSatisfied evaluates the escrow's condition against the recorded approvals and now.
func conditionSatisfied(e Escrow, approvals []string, now time.Time) bool {
	switch e.ConditionType {
	case ConditionDeliveryConfirmation:
		return contains(approvals, e.CreatorAddr)
	case ConditionTimeLock:
		return e.ReleaseAt != nil && !now.Before(*e.ReleaseAt)
	case ConditionMultiPartyApproval:
		count := 0
		for _, a := range e.ConditionParams.Approvers {
			if contains(approvals, a) {
				count++
			}
		}
		return count >= e.ConditionParams.Threshold
	}
	return false
}

func (s *Service) recordTransitions(kinds []string) {
	if s.metrics == nil {
		return
	}
	for _, k := range kinds {
		s.metrics.Transition(k)
	}
}

func (s *Service) publish(ctx context.Context, name string, e Escrow, payload map[string]any) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.Publish(ctx, name, e.PLID, payload); err != nil {
		s.log.Warn("event_publish_failed", "event", name, "escrow_id", e.ID, "err", err.Error())
	}
}

// normalizeAddr lowercases and trims an address for canonical comparison/storage (the gateway
// injects X-Creator-Addr lowercased; create inputs are normalized the same way).
func normalizeAddr(a string) string {
	return strings.ToLower(strings.TrimSpace(a))
}

// normalizeAmount validates a positive integer amount string (numeric(30,0)).
func normalizeAmount(raw string) (string, error) {
	n, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok || n.Sign() <= 0 {
		return "", errors.New("amount must be a positive integer string")
	}
	v := n.String()
	if len(v) > 30 {
		return "", errors.New("amount exceeds 30 digits")
	}
	return v, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func appendApproval(approvals []string, addr string) []string {
	if contains(approvals, addr) {
		return approvals
	}
	return append(approvals, addr)
}
