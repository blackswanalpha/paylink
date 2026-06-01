"""Provider protocol + the upstream-error type.

A ``Provider`` reads one entity kind from its owning service. The HTTP impls (``http.py``) call the
upstream admin/internal read endpoints; the in-memory ``Fake`` (``fake.py``) backs the tests. Both
normalize their upstream payloads into the rail-/service-agnostic :class:`SearchHit` /
:class:`EntityView` shapes, so the search fan-out and views never know which service answered.
"""

from __future__ import annotations

from typing import Protocol

from app.domain.models import EntityView, SearchHit


class UpstreamError(Exception):
    """An upstream read failed (network error or non-404 >= 400 response)."""

    def __init__(self, service: str, *, status: int | None = None) -> None:
        self.service = service
        self.status = status
        super().__init__(f"{service} upstream returned an error (status={status})")


class Provider(Protocol):
    entity_type: str

    async def get(self, entity_id: str) -> EntityView | None: ...

    async def search(self, q: str, limit: int) -> list[SearchHit]: ...
