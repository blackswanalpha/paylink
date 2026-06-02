// Package eventbus is LinkMint's shared Kafka client library (work15): a canonical JSON Envelope
// plus a synchronous Publisher and a commit-after-handle consumer-group Consumer. The Python
// counterpart (linkmint_eventbus) serializes byte-identically, so either language produces events
// the other consumes. Transport is Kafka via Redpanda (ADR-011); the logical event model — which
// logical name lives on which domain topic — is documented in workload/catalog.md.
//
// Delivery is at-least-once: the Publisher waits for the broker ack, and the Consumer commits an
// offset only after the handler succeeds. Duplicates are therefore possible (retry / rebalance), so
// every handler MUST be idempotent (pairs with work17). Event payloads carry ids and metadata only
// — never secrets.
package eventbus
