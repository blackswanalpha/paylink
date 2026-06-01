"""The per-process service bundle (search + entity), built from the :mod:`app.main` singletons."""

from __future__ import annotations

from dataclasses import dataclass

from app.audit.sink import AuditSink
from app.config import Settings
from app.domain.entity_service import EntityService
from app.domain.search_service import SearchService
from app.providers.registry import ProviderRegistry


@dataclass(frozen=True)
class Services:
    search: SearchService
    entity: EntityService


def build_services(registry: ProviderRegistry, audit: AuditSink, settings: Settings) -> Services:
    return Services(
        search=SearchService(
            registry,
            audit,
            timeout=settings.search_fanout_timeout_seconds,
            limit=settings.search_limit_default,
        ),
        entity=EntityService(registry, audit),
    )
