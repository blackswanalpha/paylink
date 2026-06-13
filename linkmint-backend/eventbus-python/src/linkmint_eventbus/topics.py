"""Logical-name → domain-topic mapping (the catalog model). See ``workload/catalog.md``."""

from __future__ import annotations

# The fixed set of domain topics (one per domain; the full logical name lives in the envelope).
# Kept byte-identical (same order) with eventbus-go/topics.go and the docker-compose redpanda-init
# topic list.
DOMAINS: list[str] = [
    "paylink",
    "payment",
    "chain",
    "merchant",
    "compliance",
    "identity",
    "notification",
    "escrow",
    "settlement",
    "fee",
    "pricing",
    "fx",
    "invoice",
    "refund",
    "dispute",
]


# Topic aliases: logical-name domains that share another domain's physical topic (see catalog.md).
# settlement-service (work23) publishes both ``settlement.*`` and ``payout.*`` on the settlement
# topic, so ``payout`` routes to ``settlement`` rather than a topic of its own.
_TOPIC_ALIASES: dict[str, str] = {"payout": "settlement"}


def topic_for(name: str) -> str:
    """Map a logical event name to its domain topic — the first dot-segment (with aliases).

    ``paylink.verified`` -> ``paylink``; ``chain.paylink.verified`` -> ``chain``;
    ``payout.scheduled`` -> ``settlement`` (alias).
    """
    i = name.find(".")
    segment = name[:i] if i >= 0 else name
    return _TOPIC_ALIASES.get(segment, segment)
