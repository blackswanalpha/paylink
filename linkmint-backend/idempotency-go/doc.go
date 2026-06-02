// Package idempotency is LinkMint's shared Idempotency-Key helper (work17): a Redis-backed store for
// replay-safe state-mutating HTTP endpoints (24h TTL) plus consumer-side dedupe helpers for the
// at-least-once event bus (work15).
//
// It is the APPLICATION-LAYER complement to the chain's on-chain anti-replay (invariant A.7), which
// remains the source of truth for settlement. This library never gates settlement on its Redis/DB
// marker — it only makes retries cheap and stops duplicate app-side effects (a double HTTP response, a
// double notification) BEFORE a request reaches the chain. If Redis is down or a key expires, the worst
// case is a duplicate attempt the chain's anti-replay (and the proof_hash DB UNIQUE in payment/proof)
// still rejects, so the framework fails safe toward on-chain truth. It touches neither fund flow (A.1)
// nor ledger balancing (A.6) — it only dedupes effects.
//
// # HTTP store
//
// A request that re-presents the same Idempotency-Key + body replays the cached response; the same key
// with a DIFFERENT body is an ErrConflict (map to 409 IDEMPOTENT_CONFLICT); an in-flight duplicate is
// also an ErrConflict. Keys are namespaced per service+route ("idem:<service>:<route>:<key>") so
// different routes never collide. The Python counterpart (linkmint_idempotency) uses the identical key
// scheme and JSON record shape, so Go and Python services share one Redis.
//
// # Consumer dedupe
//
// The event bus is at-least-once, so consumers MUST be idempotent. Two helpers are offered: RedisDedupe
// is a cheap best-effort short-circuit (skip repeated non-DB work); DbDedupe is the durable
// exactly-once-effect guard whose marker commits atomically with the handler's own write. The work15
// HandleFunc only delivers (name, payload) — not the envelope id — so the caller supplies a stable
// dedupe key (a proof_hash, a business id, or a payload fingerprint).
package idempotency
