package domain_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/paylink/escrow-manager/internal/domain"
	"github.com/paylink/escrow-manager/internal/fsm"
	"github.com/paylink/escrow-manager/internal/httpx"
	"github.com/paylink/escrow-manager/internal/store/memory"
)

// ---- fakes ----

type pubEvent struct {
	name    string
	key     string
	payload map[string]any
}

type fakePub struct {
	mu     sync.Mutex
	events []pubEvent
	err    error
}

func (p *fakePub) Publish(_ context.Context, name, key string, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, _ := payload.(map[string]any)
	p.events = append(p.events, pubEvent{name: name, key: key, payload: m})
	return p.err
}

func (p *fakePub) byName(name string) []pubEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []pubEvent
	for _, e := range p.events {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

type fakeMetrics struct {
	mu    sync.Mutex
	kinds []string
}

func (m *fakeMetrics) Transition(kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kinds = append(m.kinds, kind)
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// ---- helpers ----

var t0 = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

func newSvc(t *testing.T) (*domain.Service, *memory.Store, *fakePub, *fakeMetrics, *fakeClock) {
	t.Helper()
	store := memory.New()
	pub := &fakePub{}
	met := &fakeMetrics{}
	clk := &fakeClock{t: t0}
	n := 0
	svc := domain.NewService(store, pub, nil,
		domain.WithClock(clk.now),
		domain.WithIDGen(func() string { n++; return "ESC_" + string(rune('a'+n-1)) }),
		domain.WithMetrics(met),
		domain.WithDefaultTimeout(24*time.Hour),
	)
	return svc, store, pub, met, clk
}

func deliveryInput(pl string) domain.CreateInput {
	return domain.CreateInput{
		CreatorAddr:   "0xCREATOR",
		PLID:          pl,
		PayeeAddr:     "0xPAYEE",
		RefundTo:      "0xREFUND",
		Amount:        "1000",
		Currency:      "kes",
		ConditionType: domain.ConditionDeliveryConfirmation,
	}
}

func errCodeOf(t *testing.T, err error) httpx.ErrorCode {
	t.Helper()
	var ae *httpx.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want AppError, got %v", err)
	}
	return ae.Code
}

// ---- create ----

func TestCreateNormalizesAndPublishes(t *testing.T) {
	svc, _, pub, _, _ := newSvc(t)
	e, err := svc.Create(context.Background(), deliveryInput("PLK_1"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.State != fsm.StateWaiting || e.Funded {
		t.Fatalf("new escrow must be WAITING+unfunded: %+v", e)
	}
	if e.CreatorAddr != "0xcreator" || e.PayeeAddr != "0xpayee" || e.RefundTo != "0xrefund" {
		t.Fatalf("addresses must be lowercased: %+v", e)
	}
	if e.Currency != "KES" {
		t.Fatalf("currency must be uppercased: %q", e.Currency)
	}
	if !e.TimeoutAt.Equal(t0.Add(24 * time.Hour)) {
		t.Fatalf("default timeout not applied: %v", e.TimeoutAt)
	}
	created := pub.byName(domain.EventEscrowCreated)
	if len(created) != 1 || created[0].key != "PLK_1" {
		t.Fatalf("escrow.created not published: %+v", created)
	}
}

func TestCreateValidation(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	future := t0.Add(time.Hour)
	past := t0.Add(-time.Hour)
	afterTimeout := t0.Add(48 * time.Hour)

	mod := func(f func(*domain.CreateInput)) domain.CreateInput {
		in := deliveryInput("PLK_v")
		f(&in)
		return in
	}
	cases := []struct {
		name string
		in   domain.CreateInput
	}{
		{"missing pl_id", mod(func(in *domain.CreateInput) { in.PLID = "  " })},
		{"missing payee", mod(func(in *domain.CreateInput) { in.PayeeAddr = "" })},
		{"missing refund_to", mod(func(in *domain.CreateInput) { in.RefundTo = "" })},
		{"missing amount", mod(func(in *domain.CreateInput) { in.Amount = "" })},
		{"non-integer amount", mod(func(in *domain.CreateInput) { in.Amount = "10.5" })},
		{"zero amount", mod(func(in *domain.CreateInput) { in.Amount = "0" })},
		{"negative amount", mod(func(in *domain.CreateInput) { in.Amount = "-5" })},
		{"amount too long", mod(func(in *domain.CreateInput) { in.Amount = "1000000000000000000000000000000" })}, // 31 digits
		{"missing currency", mod(func(in *domain.CreateInput) { in.Currency = " " })},
		{"bad condition type", mod(func(in *domain.CreateInput) { in.ConditionType = "handshake" })},
		{"past timeout", mod(func(in *domain.CreateInput) { in.TimeoutAt = &past })},
		{"delivery with params", mod(func(in *domain.CreateInput) { in.ConditionParams = domain.ConditionParams{Threshold: 1} })},
		{"time_lock missing release_at", mod(func(in *domain.CreateInput) { in.ConditionType = domain.ConditionTimeLock })},
		{"time_lock past release_at", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionTimeLock
			in.ConditionParams = domain.ConditionParams{ReleaseAt: &past}
		})},
		{"time_lock release_at >= timeout", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionTimeLock
			in.ConditionParams = domain.ConditionParams{ReleaseAt: &afterTimeout}
		})},
		{"time_lock with approvers", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionTimeLock
			in.ConditionParams = domain.ConditionParams{ReleaseAt: &future, Approvers: []string{"0xa"}}
		})},
		{"multi_party no approvers", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionMultiPartyApproval
			in.ConditionParams = domain.ConditionParams{Threshold: 1}
		})},
		{"multi_party empty approver", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionMultiPartyApproval
			in.ConditionParams = domain.ConditionParams{Approvers: []string{"0xa", " "}, Threshold: 1}
		})},
		{"multi_party duplicate approver", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionMultiPartyApproval
			in.ConditionParams = domain.ConditionParams{Approvers: []string{"0xA", "0xa"}, Threshold: 1}
		})},
		{"multi_party threshold zero", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionMultiPartyApproval
			in.ConditionParams = domain.ConditionParams{Approvers: []string{"0xa", "0xb"}}
		})},
		{"multi_party threshold too high", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionMultiPartyApproval
			in.ConditionParams = domain.ConditionParams{Approvers: []string{"0xa", "0xb"}, Threshold: 3}
		})},
		{"multi_party with release_at", mod(func(in *domain.CreateInput) {
			in.ConditionType = domain.ConditionMultiPartyApproval
			in.ConditionParams = domain.ConditionParams{Approvers: []string{"0xa"}, Threshold: 1, ReleaseAt: &future}
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, tc.in)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if code := errCodeOf(t, err); code != httpx.CodeInvalidPayload {
				t.Fatalf("code = %s, want INVALID_PAYLOAD", code)
			}
		})
	}
}

func TestCreateDuplicatePayLink(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, deliveryInput("PLK_dup")); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, deliveryInput("PLK_dup"))
	if code := errCodeOf(t, err); code != httpx.CodeEscrowExists {
		t.Fatalf("code = %s, want ESCROW_EXISTS", code)
	}
}

// ---- delivery_confirmation ----

func TestDeliveryFundThenConfirmReleases(t *testing.T) {
	svc, _, pub, met, _ := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, deliveryInput("PLK_d1"))

	res, err := svc.HandlePaylinkVerified(ctx, "PLK_d1", "0xtx1")
	if err != nil || res != domain.ResultFunded {
		t.Fatalf("funding: res=%s err=%v", res, err)
	}
	got, err := svc.Confirm(ctx, e.ID, "0xCreator") // case-insensitive caller
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got.State != fsm.StateReleased {
		t.Fatalf("state = %s, want RELEASED", got.State)
	}
	rel := pub.byName(domain.EventEscrowReleased)
	if len(rel) != 1 {
		t.Fatalf("want exactly one escrow.released, got %d", len(rel))
	}
	// A.1: the released event is an INSTRUCTION with exactly these fields — no balances.
	want := []string{"amount", "currency", "escrow_id", "funded", "payee_addr", "pl_id", "tx_hash"}
	var keys []string
	for k := range rel[0].payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != len(want) {
		t.Fatalf("released payload keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("released payload keys = %v, want %v", keys, want)
		}
	}
	if rel[0].payload["funded"] != true || rel[0].payload["tx_hash"] != "0xtx1" {
		t.Fatalf("released payload = %v", rel[0].payload)
	}
	met.mu.Lock()
	defer met.mu.Unlock()
	if len(met.kinds) != 2 || met.kinds[0] != "conditions_met" || met.kinds[1] != "release" {
		t.Fatalf("transition kinds = %v", met.kinds)
	}
}

func TestDeliveryConfirmThenFundReleases(t *testing.T) {
	svc, _, pub, _, _ := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, deliveryInput("PLK_d2"))

	got, err := svc.Confirm(ctx, e.ID, "0xcreator")
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if got.State != fsm.StateWaiting {
		t.Fatalf("unfunded confirm must stay WAITING, got %s", got.State)
	}
	if len(pub.byName(domain.EventEscrowReleased)) != 0 {
		t.Fatal("must not release before funding")
	}
	res, err := svc.HandlePaylinkVerified(ctx, "PLK_d2", "0xtx2")
	if err != nil || res != domain.ResultReleased {
		t.Fatalf("funding after confirm: res=%s err=%v", res, err)
	}
	if len(pub.byName(domain.EventEscrowReleased)) != 1 {
		t.Fatal("escrow.released not published")
	}
}

func TestDeliveryConfirmByNonCreator(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, deliveryInput("PLK_d3"))
	_, err := svc.Confirm(ctx, e.ID, "0xpayee")
	if code := errCodeOf(t, err); code != httpx.CodeNotParticipant {
		t.Fatalf("code = %s, want NOT_PARTICIPANT", code)
	}
}

// ---- multi_party_approval ----

func multiPartyInput(pl string) domain.CreateInput {
	in := deliveryInput(pl)
	in.ConditionType = domain.ConditionMultiPartyApproval
	in.ConditionParams = domain.ConditionParams{Approvers: []string{"0xA1", "0xa2", "0xa3"}, Threshold: 2}
	return in
}

func TestMultiPartyThresholdRelease(t *testing.T) {
	svc, store, pub, _, _ := newSvc(t)
	ctx := context.Background()
	e, err := svc.Create(ctx, multiPartyInput("PLK_m1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.HandlePaylinkVerified(ctx, "PLK_m1", "0xtxm"); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Confirm(ctx, e.ID, "0xa1")
	if err != nil || got.State != fsm.StateWaiting {
		t.Fatalf("first approval: state=%s err=%v", got.State, err)
	}
	// Duplicate approval by the same approver must not advance the count.
	got, err = svc.Confirm(ctx, e.ID, "0xA1")
	if err != nil || got.State != fsm.StateWaiting {
		t.Fatalf("duplicate approval: state=%s err=%v", got.State, err)
	}
	if n := len(store.Approvals(e.ID)); n != 1 {
		t.Fatalf("approvals = %d, want 1 (idempotent PK)", n)
	}
	got, err = svc.Confirm(ctx, e.ID, "0xa3")
	if err != nil || got.State != fsm.StateReleased {
		t.Fatalf("threshold approval: state=%s err=%v", got.State, err)
	}
	if len(pub.byName(domain.EventEscrowReleased)) != 1 {
		t.Fatal("escrow.released not published")
	}
}

func TestMultiPartyNonApprover(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, multiPartyInput("PLK_m2"))
	_, err := svc.Confirm(ctx, e.ID, "0xstranger")
	if code := errCodeOf(t, err); code != httpx.CodeNotParticipant {
		t.Fatalf("code = %s, want NOT_PARTICIPANT", code)
	}
}

func TestMultiPartyApprovalsBeforeFunding(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, multiPartyInput("PLK_m3"))
	if _, err := svc.Confirm(ctx, e.ID, "0xa1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(ctx, e.ID, "0xa2"); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(ctx, e.ID, "0xcreator")
	if got.State != fsm.StateWaiting {
		t.Fatalf("threshold met but unfunded must stay WAITING, got %s", got.State)
	}
	// Funding arrives last → released in the funding transaction.
	res, err := svc.HandlePaylinkVerified(ctx, "PLK_m3", "0xtxm3")
	if err != nil || res != domain.ResultReleased {
		t.Fatalf("res=%s err=%v", res, err)
	}
}

// ---- time_lock ----

func timeLockInput(pl string, releaseAt time.Time) domain.CreateInput {
	in := deliveryInput(pl)
	in.ConditionType = domain.ConditionTimeLock
	in.ConditionParams = domain.ConditionParams{ReleaseAt: &releaseAt}
	return in
}

func TestTimeLockConfirmRejected(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	e, err := svc.Create(ctx, timeLockInput("PLK_t1", t0.Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Confirm(ctx, e.ID, "0xcreator")
	if code := errCodeOf(t, err); code != httpx.CodeConditionNotConfirmable {
		t.Fatalf("code = %s, want CONDITION_NOT_CONFIRMABLE", code)
	}
}

func TestTimeLockSweepRelease(t *testing.T) {
	svc, _, pub, _, clk := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, timeLockInput("PLK_t2", t0.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.HandlePaylinkVerified(ctx, "PLK_t2", "0xtxt"); err != nil {
		t.Fatal(err)
	}
	// Not due yet.
	svc.Sweep(ctx)
	if len(pub.byName(domain.EventEscrowReleased)) != 0 {
		t.Fatal("must not release before release_at")
	}
	clk.advance(2 * time.Hour)
	svc.Sweep(ctx)
	rel := pub.byName(domain.EventEscrowReleased)
	if len(rel) != 1 {
		t.Fatalf("want 1 release after release_at, got %d", len(rel))
	}
	// Idempotent: a second sweep does nothing (CAS already moved it off WAITING).
	svc.Sweep(ctx)
	if len(pub.byName(domain.EventEscrowReleased)) != 1 {
		t.Fatal("sweep must not double-release")
	}
}

func TestTimeLockUnfundedNotReleasedBySweep(t *testing.T) {
	svc, _, pub, _, clk := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, timeLockInput("PLK_t3", t0.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	clk.advance(2 * time.Hour)
	svc.Sweep(ctx)
	if len(pub.byName(domain.EventEscrowReleased)) != 0 {
		t.Fatal("unfunded time_lock must never release")
	}
}

func TestTimeLockFundingAfterReleaseAtReleasesImmediately(t *testing.T) {
	svc, _, _, _, clk := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, timeLockInput("PLK_t4", t0.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	clk.advance(2 * time.Hour)
	res, err := svc.HandlePaylinkVerified(ctx, "PLK_t4", "0xtxt4")
	if err != nil || res != domain.ResultReleased {
		t.Fatalf("res=%s err=%v", res, err)
	}
}

// ---- timeout ----

func TestTimeoutRefundUnfunded(t *testing.T) {
	svc, _, pub, met, clk := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, deliveryInput("PLK_to1")); err != nil {
		t.Fatal(err)
	}
	clk.advance(25 * time.Hour)
	svc.Sweep(ctx)
	ref := pub.byName(domain.EventEscrowRefunded)
	if len(ref) != 1 {
		t.Fatalf("want 1 refund, got %d", len(ref))
	}
	// A.1: refund instruction payload — exact fields, funded:false (nothing was ever paid).
	want := []string{"amount", "currency", "escrow_id", "funded", "pl_id", "refund_to", "tx_hash"}
	var keys []string
	for k := range ref[0].payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i := range want {
		if len(keys) != len(want) || keys[i] != want[i] {
			t.Fatalf("refunded payload keys = %v, want %v", keys, want)
		}
	}
	if ref[0].payload["funded"] != false || ref[0].payload["refund_to"] != "0xrefund" {
		t.Fatalf("refunded payload = %v", ref[0].payload)
	}
	met.mu.Lock()
	defer met.mu.Unlock()
	if len(met.kinds) != 1 || met.kinds[0] != "timeout" {
		t.Fatalf("transition kinds = %v", met.kinds)
	}
}

func TestTimeoutRefundFundedCarriesFlag(t *testing.T) {
	svc, _, pub, _, clk := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, multiPartyInput("PLK_to2")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.HandlePaylinkVerified(ctx, "PLK_to2", "0xtxo"); err != nil {
		t.Fatal(err)
	}
	clk.advance(25 * time.Hour)
	svc.Sweep(ctx)
	ref := pub.byName(domain.EventEscrowRefunded)
	if len(ref) != 1 || ref[0].payload["funded"] != true {
		t.Fatalf("funded refund payload = %+v", ref)
	}
}

// ---- dispute ----

func TestDisputeFlow(t *testing.T) {
	svc, _, pub, _, clk := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, deliveryInput("PLK_x1"))

	if _, err := svc.Dispute(ctx, e.ID, "0xstranger", "where is my stuff"); errCodeOf(t, err) != httpx.CodeNotParticipant {
		t.Fatal("non-participant must not dispute")
	}
	if _, err := svc.Dispute(ctx, e.ID, "0xpayee", "  "); errCodeOf(t, err) != httpx.CodeInvalidPayload {
		t.Fatal("empty reason must be rejected")
	}
	got, err := svc.Dispute(ctx, e.ID, "0xpayee", "no delivery")
	if err != nil || got.State != fsm.StateDisputed || got.DisputeReason != "no delivery" {
		t.Fatalf("dispute: %+v err=%v", got, err)
	}
	if len(pub.byName(domain.EventEscrowDisputed)) != 1 {
		t.Fatal("escrow.disputed not published")
	}

	// DISPUTED is terminal here: re-dispute and confirm both 409; sweeper and consumer skip it.
	if _, err := svc.Dispute(ctx, e.ID, "0xcreator", "again"); errCodeOf(t, err) != httpx.CodeInvalidState {
		t.Fatal("re-dispute must be INVALID_STATE")
	}
	if _, err := svc.Confirm(ctx, e.ID, "0xcreator"); errCodeOf(t, err) != httpx.CodeInvalidState {
		t.Fatal("confirm after dispute must be INVALID_STATE")
	}
	clk.advance(48 * time.Hour)
	svc.Sweep(ctx)
	if len(pub.byName(domain.EventEscrowRefunded)) != 0 {
		t.Fatal("sweeper must not touch DISPUTED escrows")
	}
	res, err := svc.HandlePaylinkVerified(ctx, "PLK_x1", "0xlate")
	if err != nil || res != domain.ResultSkipped {
		t.Fatalf("funding a DISPUTED escrow: res=%s err=%v", res, err)
	}
	got, _ = svc.Get(ctx, e.ID, "0xcreator")
	if got.Funded {
		t.Fatal("DISPUTED escrow must not be marked funded")
	}
}

// ---- consumer-facing handle ----

func TestHandlePaylinkVerifiedResults(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()

	if res, err := svc.HandlePaylinkVerified(ctx, "PLK_unknown", "0xtx"); err != nil || res != domain.ResultIgnored {
		t.Fatalf("unknown paylink: res=%s err=%v", res, err)
	}

	e, _ := svc.Create(ctx, deliveryInput("PLK_h1"))
	if res, _ := svc.HandlePaylinkVerified(ctx, "PLK_h1", "0xtx1"); res != domain.ResultFunded {
		t.Fatalf("first funding: res=%s", res)
	}
	if res, _ := svc.HandlePaylinkVerified(ctx, "PLK_h1", "0xtx1"); res != domain.ResultDuplicate {
		t.Fatalf("redelivery: res=%s", res)
	}
	got, _ := svc.Get(ctx, e.ID, "0xcreator")
	if !got.Funded || got.FundedTxHash != "0xtx1" {
		t.Fatalf("funded flag/tx: %+v", got)
	}

	// Release, then a NEW tx hash for the same paylink is recorded as processed but skipped.
	if _, err := svc.Confirm(ctx, e.ID, "0xcreator"); err != nil {
		t.Fatal(err)
	}
	if res, _ := svc.HandlePaylinkVerified(ctx, "PLK_h1", "0xtx2"); res != domain.ResultSkipped {
		t.Fatalf("late event: res=%s", res)
	}
}

// ---- reads ----

func TestGetNotFound(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	_, err := svc.Get(context.Background(), "ESC_missing", "0xcreator")
	if code := errCodeOf(t, err); code != httpx.CodeEscrowNotFound {
		t.Fatalf("code = %s, want ESCROW_NOT_FOUND", code)
	}
}

func TestGetViewScoping(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	e, _ := svc.Create(ctx, multiPartyInput("PLK_view"))

	// Outsiders get the same 404 as a missing id — no existence leak.
	_, err := svc.Get(ctx, e.ID, "0xstranger")
	if code := errCodeOf(t, err); code != httpx.CodeEscrowNotFound {
		t.Fatalf("outsider code = %s, want ESCROW_NOT_FOUND", code)
	}
	// Participants and listed approvers can view (caller normalized like Confirm).
	for _, addr := range []string{"0xcreator", "0xpayee", "0xrefund", "0xa1", "0xA1"} {
		if _, err := svc.Get(ctx, e.ID, addr); err != nil {
			t.Fatalf("viewer %s: %v", addr, err)
		}
	}
}

func TestConfirmNotFound(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	_, err := svc.Confirm(context.Background(), "ESC_missing", "0xcreator")
	if code := errCodeOf(t, err); code != httpx.CodeEscrowNotFound {
		t.Fatalf("code = %s, want ESCROW_NOT_FOUND", code)
	}
	_, err = svc.Dispute(context.Background(), "ESC_missing", "0xcreator", "r")
	if code := errCodeOf(t, err); code != httpx.CodeEscrowNotFound {
		t.Fatalf("dispute code = %s, want ESCROW_NOT_FOUND", code)
	}
}

func TestListCreatorScopedAndFiltered(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, deliveryInput("PLK_l1")); err != nil {
		t.Fatal(err)
	}
	other := deliveryInput("PLK_l2")
	other.CreatorAddr = "0xother"
	if _, err := svc.Create(ctx, other); err != nil {
		t.Fatal(err)
	}

	mine, err := svc.List(ctx, "0xCREATOR", "", 20)
	if err != nil || len(mine) != 1 || mine[0].PLID != "PLK_l1" {
		t.Fatalf("creator scope: %+v err=%v", mine, err)
	}
	waiting, err := svc.List(ctx, "0xcreator", "waiting", 20)
	if err != nil || len(waiting) != 1 {
		t.Fatalf("state filter (case-insensitive): %+v err=%v", waiting, err)
	}
	none, err := svc.List(ctx, "0xcreator", "RELEASED", 20)
	if err != nil || len(none) != 0 {
		t.Fatalf("released filter: %+v err=%v", none, err)
	}
	if _, err := svc.List(ctx, "0xcreator", "BOGUS", 20); errCodeOf(t, err) != httpx.CodeInvalidPayload {
		t.Fatal("invalid state filter must be INVALID_PAYLOAD")
	}
}

func TestReady(t *testing.T) {
	svc, _, _, _, _ := newSvc(t)
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("Ready: %v", err)
	}
}

// TestPublishFailureDoesNotFailRequest ensures the publish seam is fire-and-warn.
func TestPublishFailureDoesNotFailRequest(t *testing.T) {
	store := memory.New()
	pub := &fakePub{err: errors.New("broker down")}
	clk := &fakeClock{t: t0}
	svc := domain.NewService(store, pub, nil, domain.WithClock(clk.now))
	if _, err := svc.Create(context.Background(), deliveryInput("PLK_p1")); err != nil {
		t.Fatalf("Create must succeed despite publish failure: %v", err)
	}
}
