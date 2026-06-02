"""Logical-name → domain-topic mapping (the catalog model). See ``workload/catalog.md``."""

from __future__ import annotations

# The fixed set of domain topics (one per domain; the full logical name lives in the envelope).
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
]


def topic_for(name: str) -> str:
    """Map a logical event name to its domain topic — the first dot-segment.

    ``paylink.verified`` -> ``paylink``; ``chain.paylink.verified`` -> ``chain``.
    """
    i = name.find(".")
    return name[:i] if i >= 0 else name
