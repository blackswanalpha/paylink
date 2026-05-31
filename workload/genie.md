# Genie — turn a vague ask into a structured prompt

A one-line request like *"add stripe"* forces Claude to guess scope, constraints, and
done-criteria. The Genie restructures any raw ask into a prompt Claude can execute well.
Use it before kicking off a non-trivial task, or via `/new-work` when seeding a backlog item.

## The structure

Fill these six fields. Keep each tight; link to repo docs instead of re-explaining them.

1. **Context** — Where this fits. The relevant work item, the current state of the code,
   the design doc section. *Why now.*
2. **Task** — One sentence: the outcome, in imperative form.
3. **Constraints / invariants** — Which [`rules.md`](rules.md) invariants apply, plus any
   [`scope.md`](scope.md) fences. Call out what's explicitly out of scope.
4. **Reuse-first pointers** — Existing functions, types, packages, or patterns to build on
   (`paylink-chain/internal/...`, a sibling service, an SDK client).
5. **Acceptance criteria** — Bullet list of what must be true to call it done. Testable.
6. **Verification** — The exact commands / steps to prove it works
   ([`verification.md`](verification.md)).

## Template (also in [`templates/prompt.template.md`](templates/prompt.template.md))

```
Context:      <work item + current state + design ref>
Task:         <one imperative sentence>
Constraints:  <invariants from rules.md; out-of-scope from scope.md>
Reuse:        <existing functions/types/packages to build on>
Acceptance:   - <testable bullet>
              - <testable bullet>
Verify:       <commands / steps from verification.md>
```

---

## Worked examples

### 1. Raw: "add stripe"
```
Context:      Card rail for PayLink settlement. work04 (MPesa) establishes the adapter
              pattern; this is a future item (see scope.md — card is deferred, so this is
              a NEW backlog item, not part of work04). Pattern ref: adapters/, flow04.
Task:         Implement a Stripe card adapter that settles a PayLink from a card payment.
Constraints:  Non-custodial (rules A.1) — funds go buyer→merchant via Stripe, LinkMint
              never holds them; output ONLY the rail-agnostic proof (rules A.4); no
              Stripe-specific fields past the adapter boundary.
Reuse:        The receive→normalize→sign→broadcast pipeline from the MPesa adapter; the
              proof signing in paylink-chain/internal/crypto; proof shape from standard.md.
Acceptance:   - Stripe webhook verified, payment mapped to {pl_id, rail:"card", ...}.
              - Proof signed and broadcast to the validator; PayLink settles on-chain.
              - Unit + integration tests; 80% coverage; lint clean.
Verify:       Replay a captured Stripe webhook → observe proof broadcast → query RPC for
              settled PayLink (verification.md "adapter" + "full stack").
```

### 2. Raw: "the chain should support refunds"
```
Context:      New PayLink lifecycle transition (refund). Chain work in paylink-chain.
Task:         Add a refund transaction type to the lVM executor.
Constraints:  No EVM/contracts (rules A.2) — native executor only; preserve anti-replay
              (A.7) and double-entry ledger (A.6); refunds must not create custody (A.1).
Reuse:        The tx-type recipe in standard.md; existing settle path in
              internal/chain/executor.go; FSM transitions in internal/fsm; events in
              internal/events/event.go.
Acceptance:   - New constant + payload + executor case + event kinds.
              - FSM allows settled→refunded only under documented rules.
              - Table-driven + integration tests; deterministic state root.
Verify:       cd paylink-chain && go build ./... && go test ./... -count=1.
```

### 3. Raw: "make a frontend"
```
Context:      MVP web flow (work07). Depends on the API gateway (work05) + JS SDK (work06).
Task:         Build a React page to create a PayLink, pay via MPesa, and show settlement.
Constraints:  TS strict, no `any` (standard.md); call the API only through the JS SDK,
              not raw fetch; no card/crypto UI (scope.md — MPesa only this phase).
Reuse:        The JS SDK client (work06); the /v1 endpoints; error envelope shape.
Acceptance:   - Create-PayLink form → shows pay instructions → live settlement status.
              - Handles the standard error envelope; basic loading/error states.
Verify:       npm run dev against local docker-compose stack; create→pay→settle works.
```

### 4. Raw: "speed up block production"
```
Context:      Performance tuning of the existing producer. paylink-chain consensus.
Task:         Reduce block production latency without weakening consensus.
Constraints:  Keep PoV + VRF committee + 3-of-5 quorum + immediate finality (rules A.3);
              keep determinism (standard.md); no random in state paths.
Reuse:        internal/consensus block producer; existing benchmarks/tests if present.
Acceptance:   - Measured latency improvement; all consensus tests still pass.
              - No change to committee selection semantics.
Verify:       go test ./internal/consensus/... -count=1; run paylinkd --dev and observe.
```

---

**Rule of thumb:** if you can't fill in *Acceptance* and *Verify*, the task isn't ready —
clarify scope first (ask the user, or split it in [`backlog.md`](backlog.md)).
