"""In-memory provider for tests — same Protocol surface as the HTTP impls.

Seed it with ``{entity_id: payload}``; ``search`` matches an exact id or any substring of the
payload values. Set ``fail=True`` to simulate an upstream outage (raises :class:`UpstreamError`),
which exercises the search degrade-path and the entity-view 502.
"""

from __future__ import annotations

from typing import Any

from app.domain.models import EntityView, SearchHit
from app.providers.base import UpstreamError


class FakeProvider:
    def __init__(
        self,
        entity_type: str,
        *,
        entities: dict[str, dict[str, Any]] | None = None,
        fail: bool = False,
    ) -> None:
        self.entity_type = entity_type
        self._entities = entities or {}
        self._fail = fail

    async def get(self, entity_id: str) -> EntityView | None:
        if self._fail:
            raise UpstreamError(self.entity_type, status=503)
        data = self._entities.get(entity_id)
        return None if data is None else EntityView(self.entity_type, entity_id, data)

    async def search(self, q: str, limit: int) -> list[SearchHit]:
        if self._fail:
            raise UpstreamError(self.entity_type, status=503)
        ql = q.lower()
        hits: list[SearchHit] = []
        for eid, data in self._entities.items():
            blob = " ".join(str(v) for v in data.values()).lower()
            if ql == eid.lower() or ql in blob:
                hits.append(
                    SearchHit(
                        type=self.entity_type,
                        id=eid,
                        label=str(data.get("label", eid)),
                        status=data.get("status"),
                    )
                )
        return hits[:limit]
