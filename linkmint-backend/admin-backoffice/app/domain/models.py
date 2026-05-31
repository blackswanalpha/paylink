"""Domain value objects (pure, no I/O).

``SearchHit``/``EntityView`` are the normalized shapes every provider maps its upstream payload
into, so the search fan-out and the entity views are rail-/service-agnostic. ``SCOPES`` is the
closed set of admin permissions (spec §2.18); authorization is default-deny against it.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

# Granular admin permissions (default-deny). `superuser` implies all of the others.
SCOPES: frozenset[str] = frozenset(
    {
        "support.read",
        "support.write",
        "finance.refund",
        "compliance.resolve",
        "engineer.feature_flags",
        "superuser",
    }
)

# The four entity kinds the console reads. Order defines the search fan-out order.
ENTITY_TYPES: tuple[str, ...] = ("user", "merchant", "paylink", "payment")


@dataclass(frozen=True)
class SearchHit:
    """A single unified-search result, normalized across services."""

    type: str  # one of ENTITY_TYPES
    id: str
    label: str
    status: str | None = None
    secondary: dict[str, str] = field(default_factory=dict)


@dataclass(frozen=True)
class EntityView:
    """A drill-down read view — the upstream payload, tagged with its type/id."""

    type: str
    id: str
    data: dict[str, Any]


@dataclass(frozen=True)
class SearchResult:
    """The unified-search response: hits grouped by type + the list of degraded providers."""

    query: str
    groups: dict[str, list[SearchHit]]
    degraded: list[str]
