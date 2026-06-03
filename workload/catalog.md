# Domain Event Catalog

The logical event model for LinkMint's asynchronous backbone (work15). Services reference events
by their **logical name** (e.g. `paylink.verified`); the transport is **Kafka via Redpanda** with
**one topic per domain** (see [ADR-011](decisions.md), which refines ADR-004). This file is the
contract — a service author wiring a publisher/consumer looks here for the name, topic, and who
else is on the wire.

## Model

- **Logical name** = `<domain>.<event>` (or `<domain>.<entity>.<event>` for chain). It is carried
  in the envelope `name` field — *not* encoded in the topic.
- **Topic** = the domain (the first dot-segment). Consumers subscribe to a **topic** and dispatch
  by `name`. This mirrors the backendfeatures.md "stream/subject" taxonomy: a topic is a stream,
  a logical name is a subject.
- **Key** = the entity id (`pl_id`, `user_id`, `payment_id`, …). Kafka partitions by key, so all
  events for one entity stay ordered.
- **Envelope** (byte-identical across the Go and Python client libs):
  ```json
  { "id": "<uuid>", "name": "paylink.verified", "key": "PLK_…",
    "correlation_id": "<trace_id>", "occurred_at": "2026-06-01T12:00:00Z",
    "source": "paylink-service", "payload": { … } }
  ```
- **Delivery** = at-least-once (producer waits for broker ack; consumer commits offset only after
  `handle()` succeeds). **Consumers MUST be idempotent** (duplicates happen on retry/rebalance) —
  pairs with work17; notification-service's per-event DB dedupe is the reference.
- **No secrets in payloads.** Ids and metadata only — never passwords, API keys, hashes, refresh
  tokens, MFA secrets, card PANs, or full KYC documents (redact at the producer boundary).

## Topics

`paylink` · `payment` · `chain` · `merchant` · `compliance` · `identity` · `notification` ·
`escrow` · `settlement` · `fee` · `pricing` · `fx` · `invoice` (13 domain topics; 1 partition / 1
replica locally). The `pricing`/`fx`/`invoice` topics carry the fee-pricing (work21) and
invoice-subscription (work19) events: per the model below, the topic is the logical name's first
dot-segment, so `pricing.fee_quote.issued` → `pricing`, `fx.rate.updated` → `fx`,
`invoice.platform_fee.issued` / `invoice.*` → `invoice`. (`fee` is reserved for future fee-domain
events whose name starts `fee.`.)

## Catalog

| Logical name | Topic | Producer | Consumers | Phase |
|---|---|---|---|---|
| `paylink.requested` | paylink | paylink-service | compliance-risk, payment-orchestrator | 1 |
| `paylink.created` | paylink | paylink-service | (internal) | 1 |
| `paylink.cancelled` | paylink | paylink-service | (internal) | 1 |
| `paylink.expired` | paylink | paylink-service (sweeper) | notification-service | 1 |
| `paylink.verified` | paylink | paylink-service | notification-service | 1 |
| `payment.initiated` | payment | payment-orchestrator | compliance-risk | 1 |
| `payment.proof_received` | payment | payment-orchestrator | proof-validator | 1 |
| `payment.failed` | payment | payment-orchestrator | notification-service | 1 |
| `payment.timeout` | payment | payment-orchestrator | (internal) | 1 |
| `payment.proof_validated` | payment | proof-validator | payment-orchestrator | 1 |
| `payment.proof_rejected` | payment | proof-validator | payment-orchestrator | 1 |
| `chain.paylink.created` | chain | **chain-event-mirror** | paylink-service | 1 |
| `chain.paylink.verified` | chain | **chain-event-mirror** | paylink-service, payment-orchestrator, escrow-manager, settlement-service | 1/2 |
| `chain.paylink.cancelled` | chain | **chain-event-mirror** | paylink-service | 1 |
| `chain.paylink.failed` | chain | **chain-event-mirror** | paylink-service | 1 |
| `chain.paylink.voted` | chain | **chain-event-mirror** | proof-validator | 1 |
| `chain.paylink.transferred` | chain | **chain-event-mirror** | paylink-service | 2 |
| `chain.paylink.approved` / `chain.paylink.approval_for_all` | chain | **chain-event-mirror** | (internal) | 2 |
| `chain.validator.*` (staked/activated/deactivated/unstake_started/unstake_completed/slashed/rewarded/vrf_key_registered) | chain | **chain-event-mirror** | (observability/audit) | 1/2 |
| `chain.fee.collected` / `chain.fee.distributed` / `chain.token.burned` | chain | **chain-event-mirror** | settlement-service | 2 |
| `chain.account.transfer` | chain | **chain-event-mirror** | (observability) | 1 |
| `chain.block.produced` | chain | **chain-event-mirror** | (observability) | 1 |
| `chain.tx.executed` / `chain.tx.failed` | chain | **chain-event-mirror** | (observability/audit) | 1 |
| `merchant.onboarded` | merchant | merchant-onboarding | settlement-service, fee-pricing-service | 1 |
| `merchant.verified` / `merchant.rejected` / `merchant.suspended` | merchant | merchant-onboarding | notification-service | 1 |
| `merchant.bank_account.added` / `merchant.bank_account.verified` | merchant | merchant-onboarding | (internal) | 1 |
| `merchant.contract.accepted` / `merchant.fee_tier.changed` | merchant | merchant-onboarding | fee-pricing-service | 1 |
| `identity.user.registered` … `identity.auth.failed` (registered/verified/suspended; org.created; member.added/removed; api_key.issued/revoked; mfa.enabled; auth.failed) | identity | identity-service | notification-service, audit-log-service | 1 |
| `compliance.kyc.passed` / `compliance.kyc.failed` | compliance | compliance-risk | identity-service | 2 |
| `compliance.kyb.passed` / `compliance.kyb.failed` | compliance | compliance-risk | merchant-onboarding | 2 |
| `compliance.check.passed` / `compliance.check.failed` | compliance | compliance-risk | paylink-service | 2 |
| `compliance.flag.raised` | compliance | compliance-risk | admin/audit | 2 |
| `notification.delivered` / `notification.bounced` | notification | notification-service | (reporting) | 2 |
| `escrow.created` / `escrow.released` / `escrow.refunded` / `escrow.disputed` | escrow | escrow-manager | notification-service | 2 |
| `settlement.batch_created` / `settlement.completed` / `payout.*` | settlement | settlement-service | reporting, reconciliation | 2 |
| `pricing.fee_quote.issued` / `fx.rate.updated` / `invoice.platform_fee.issued` | pricing / fx / invoice | fee-pricing-service | invoice-subscription | 2 |

> **Phase-2 rows** (escrow / settlement / fee / refund / most compliance) are the **contract ahead
> of the service**: the producing service is not yet built (see [backlog.md](backlog.md)), but the
> name/topic are fixed here so a future implementer and its consumers agree up front.

## The `chain.*` family

The `chain` topic is fed exclusively by **chain-event-mirror**, which subscribes to the lVM
WebSocket datastream (`paylink-chain/internal/datastream`) and republishes each chain event as
`chain.<kind>`. The full kind set is enumerated from
`paylink-chain/internal/events/event.go` (`EventKind` constants): every `paylink.*`, `validator.*`,
`fee.*`, `token.*`, `account.*`, `block.*`, and `tx.*` kind becomes `chain.<kind>` on the `chain`
topic, keyed by the event's `entityId`. The mirror's `CEM_CHAIN_EVENT_KINDS` env filter can
restrict this set (default = all; production should narrow to the settlement-relevant kinds to keep
high-volume `chain.block.produced` / `chain.tx.executed` off the bus).
