"""Unified search — fan out ``q`` across every provider, group by type, degrade gracefully.

Each provider runs under its own timeout so one slow/failing upstream degrades only its own group
(recorded in ``degraded``) — the response is still 200 with whatever other providers returned. Every
search emits exactly one audit record (the only call site for ``admin.search``).
"""

from __future__ import annotations

import asyncio

from app.audit.sink import AuditRecord, AuditSink
from app.domain.models import SearchResult
from app.errors import AppError, ErrorCode
from app.logging import trace_id_var
from app.providers.base import Provider
from app.providers.registry import ProviderRegistry
from app.security.jwt import AccessClaims

_MIN_QUERY_LEN = 2


class SearchService:
    def __init__(
        self, registry: ProviderRegistry, audit: AuditSink, *, timeout: float, limit: int
    ) -> None:
        self._registry = registry
        self._audit = audit
        self._timeout = timeout
        self._limit = limit

    async def search(
        self,
        *,
        principal: AccessClaims,
        scopes: frozenset[str],
        ip: str | None,
        user_agent: str | None,
        q: str,
    ) -> SearchResult:
        query = q.strip()
        if len(query) < _MIN_QUERY_LEN:
            raise AppError(
                ErrorCode.INVALID_QUERY, f"q must be at least {_MIN_QUERY_LEN} characters"
            )

        providers = self._registry.all()
        outcomes = await asyncio.gather(*(self._run(p, query) for p in providers))
        groups: dict[str, list] = {}
        degraded: list[str] = []
        for etype, hits, failed in outcomes:
            if failed:
                degraded.append(etype)
            else:
                groups[etype] = hits

        await self._audit.emit(
            AuditRecord(
                actor=principal.user_id,
                actor_scopes=sorted(scopes),
                action="admin.search",
                resource_type="search",
                resource_id=None,
                query=query,
                result="degraded" if degraded else "ok",
                trace_id=trace_id_var.get(),
                ip=ip,
                user_agent=user_agent,
            )
        )
        return SearchResult(query=query, groups=groups, degraded=sorted(degraded))

    async def _run(self, provider: Provider, q: str) -> tuple[str, list, bool]:
        try:
            hits = await asyncio.wait_for(provider.search(q, self._limit), self._timeout)
            return provider.entity_type, hits, False
        except Exception:  # noqa: BLE001 - any provider failure degrades only that group
            return provider.entity_type, [], True
