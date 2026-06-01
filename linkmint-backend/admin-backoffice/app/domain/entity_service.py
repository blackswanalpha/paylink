"""Entity drill-down reads. Each view fetches from the owning provider and emits one audit record.

A provider ``None`` (404 upstream) becomes ``ENTITY_NOT_FOUND``; an :class:`UpstreamError` becomes
``UPSTREAM_UNAVAILABLE`` (502). Either way the access is audited (result ``not_found`` / ``error``).
"""

from __future__ import annotations

from app.audit.sink import AuditRecord, AuditSink
from app.domain.models import EntityView
from app.errors import AppError, ErrorCode
from app.logging import trace_id_var
from app.providers.base import UpstreamError
from app.providers.registry import ProviderRegistry
from app.security.jwt import AccessClaims


class EntityService:
    def __init__(self, registry: ProviderRegistry, audit: AuditSink) -> None:
        self._registry = registry
        self._audit = audit

    async def get(
        self,
        *,
        principal: AccessClaims,
        scopes: frozenset[str],
        ip: str | None,
        user_agent: str | None,
        entity_type: str,
        entity_id: str,
    ) -> EntityView:
        provider = self._registry.provider_for(entity_type)
        try:
            view = await provider.get(entity_id)
        except UpstreamError as exc:
            await self._emit(principal, scopes, entity_type, entity_id, "error", ip, user_agent)
            raise AppError(
                ErrorCode.UPSTREAM_UNAVAILABLE,
                f"{exc.service} is unavailable",
                details={"service": exc.service},
            ) from exc

        await self._emit(
            principal,
            scopes,
            entity_type,
            entity_id,
            "ok" if view is not None else "not_found",
            ip,
            user_agent,
        )
        if view is None:
            raise AppError(
                ErrorCode.ENTITY_NOT_FOUND,
                f"{entity_type} not found",
                details={"id": entity_id},
            )
        return view

    async def _emit(
        self,
        principal: AccessClaims,
        scopes: frozenset[str],
        entity_type: str,
        entity_id: str,
        result: str,
        ip: str | None,
        user_agent: str | None,
    ) -> None:
        await self._audit.emit(
            AuditRecord(
                actor=principal.user_id,
                actor_scopes=sorted(scopes),
                action=f"admin.view.{entity_type}",
                resource_type=entity_type,
                resource_id=entity_id,
                query=None,
                result=result,
                trace_id=trace_id_var.get(),
                ip=ip,
                user_agent=user_agent,
            )
        )
