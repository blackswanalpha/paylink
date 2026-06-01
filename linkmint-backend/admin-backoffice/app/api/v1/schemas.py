"""Response models for the admin API."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field

from app.domain.models import EntityView, SearchHit, SearchResult


class SearchHitModel(BaseModel):
    type: str
    id: str
    label: str
    status: str | None = None
    secondary: dict[str, str] = Field(default_factory=dict)

    @classmethod
    def from_hit(cls, hit: SearchHit) -> SearchHitModel:
        return cls(
            type=hit.type, id=hit.id, label=hit.label, status=hit.status, secondary=hit.secondary
        )


class SearchResponse(BaseModel):
    query: str
    groups: dict[str, list[SearchHitModel]]
    degraded: list[str]

    @classmethod
    def from_result(cls, result: SearchResult) -> SearchResponse:
        return cls(
            query=result.query,
            groups={
                etype: [SearchHitModel.from_hit(h) for h in hits]
                for etype, hits in result.groups.items()
            },
            degraded=result.degraded,
        )


class EntityResponse(BaseModel):
    type: str
    id: str
    data: dict[str, Any]

    @classmethod
    def from_view(cls, view: EntityView) -> EntityResponse:
        return cls(type=view.type, id=view.id, data=view.data)
