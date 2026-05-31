"""Scoped API keys: issue (full key shown once), list (hash hidden), revoke, verify.

identity-service is the system of record for keys. ``verify`` is implemented and tested but not yet
consumed by the gateway (Kong keeps its declarative partner key today) — wiring identity as the key
authority is a work05 follow-up.
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime

from app.db.models import ApiKeyRow
from app.db.repositories import IdentityRepository
from app.domain import rbac
from app.domain.models import ApiKeyStatus, Permission
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.api_keys import generate_api_key, prefix_of, verify_api_key
from app.security.jwt import OrgRole
from app.security.passwords import PasswordHashing

_Commit = Callable[[], Awaitable[None]]


class ApiKeysService:
    def __init__(
        self,
        repo: IdentityRepository,
        commit: _Commit,
        passwords: PasswordHashing,
        publisher: Publisher,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._passwords = passwords
        self._publisher = publisher

    async def _actor_roles(self, actor_id: uuid.UUID) -> list[OrgRole]:
        """Fresh org roles for RBAC — never trust the (possibly stale) token claims."""
        memberships = await self._repo.list_memberships_for_user(actor_id)
        return [OrgRole(org_id=str(m.org_id), role=m.role) for m in memberships]

    async def issue(
        self, *, actor_id: uuid.UUID, org_id: uuid.UUID, name: str, scopes: list[str]
    ) -> tuple[ApiKeyRow, str]:
        """Issue a scoped key. Returns ``(row, full_key)`` — the full key is shown only here."""
        role = rbac.require(await self._actor_roles(actor_id), str(org_id), Permission.APIKEY_ISSUE)
        rbac.validate_scopes(role, scopes)
        generated = generate_api_key(self._passwords)
        row = ApiKeyRow(
            api_key_id=uuid.uuid4(),
            org_id=org_id,
            name=name,
            prefix=generated.prefix,
            hash=generated.hash,
            scopes=scopes,
            status=ApiKeyStatus.ACTIVE.value,
        )
        await self._repo.insert_api_key(row)
        await self._repo.add_event(
            "api_key",
            row.api_key_id,
            events.API_KEY_ISSUED,
            {"api_key_id": str(row.api_key_id), "org_id": str(org_id), "scopes": scopes},
        )
        await self._commit()
        await self._publisher.publish(
            events.API_KEY_ISSUED,
            {"api_key_id": str(row.api_key_id), "org_id": str(org_id), "scopes": scopes},
        )
        return row, generated.full_key

    async def list_for_actor(self, *, actor_id: uuid.UUID) -> list[ApiKeyRow]:
        out: list[ApiKeyRow] = []
        for r in await self._actor_roles(actor_id):
            if rbac.can(r.role, Permission.APIKEY_READ):
                out.extend(await self._repo.list_api_keys_for_org(uuid.UUID(r.org_id)))
        return out

    async def revoke(self, *, actor_id: uuid.UUID, api_key_id: uuid.UUID) -> ApiKeyRow:
        row = await self._repo.get_api_key(api_key_id)
        if row is None:
            raise AppError(ErrorCode.API_KEY_NOT_FOUND, "API key not found")
        rbac.require(await self._actor_roles(actor_id), str(row.org_id), Permission.APIKEY_REVOKE)
        if row.status != ApiKeyStatus.REVOKED:
            row.status = ApiKeyStatus.REVOKED.value
            row.revoked_at = datetime.now(UTC)
            await self._repo.add_event(
                "api_key",
                row.api_key_id,
                events.API_KEY_REVOKED,
                {"api_key_id": str(row.api_key_id), "org_id": str(row.org_id)},
            )
            await self._commit()
            await self._publisher.publish(
                events.API_KEY_REVOKED,
                {"api_key_id": str(row.api_key_id), "org_id": str(row.org_id)},
            )
        return row

    async def verify(self, full_key: str) -> ApiKeyRow | None:
        """Resolve a presented full key to its active row (gateway seam; not yet wired)."""
        for row in await self._repo.list_api_keys_by_prefix(prefix_of(full_key)):
            if row.status == ApiKeyStatus.ACTIVE and verify_api_key(
                self._passwords, full_key, row.hash
            ):
                return row
        return None
