"""Organizations + membership CRUD with RBAC enforcement."""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable

from app.db.models import MembershipRow, OrganizationRow
from app.db.repositories import IdentityRepository
from app.domain import rbac
from app.domain.models import OrgType, Permission, Role
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.jwt import OrgRole

_Commit = Callable[[], Awaitable[None]]


class OrgsService:
    def __init__(self, repo: IdentityRepository, commit: _Commit, publisher: Publisher) -> None:
        self._repo = repo
        self._commit = commit
        self._publisher = publisher

    async def _actor_roles(self, actor_id: uuid.UUID) -> list[OrgRole]:
        """Fresh org roles for RBAC — never trust the (possibly stale) token claims."""
        memberships = await self._repo.list_memberships_for_user(actor_id)
        return [OrgRole(org_id=str(m.org_id), role=m.role) for m in memberships]

    async def create(
        self, *, creator_id: uuid.UUID, name: str, org_type: str
    ) -> tuple[OrganizationRow, MembershipRow]:
        if org_type not in set(OrgType):
            raise AppError(ErrorCode.INVALID_PAYLOAD, f"invalid org type '{org_type}'")
        org = OrganizationRow(org_id=uuid.uuid4(), name=name, type=org_type, created_by=creator_id)
        await self._repo.insert_org(org)
        membership = MembershipRow(org_id=org.org_id, user_id=creator_id, role=Role.OWNER.value)
        await self._repo.insert_membership(membership)
        await self._repo.add_event(
            "org", org.org_id, events.ORG_CREATED, {"org_id": str(org.org_id), "type": org_type}
        )
        await self._commit()
        await self._publisher.publish(
            events.ORG_CREATED, {"org_id": str(org.org_id), "created_by": str(creator_id)}
        )
        await self._publisher.publish(
            events.MEMBER_ADDED,
            {"org_id": str(org.org_id), "user_id": str(creator_id), "role": Role.OWNER.value},
        )
        return org, membership

    async def add_member(
        self,
        *,
        actor_id: uuid.UUID,
        org_id: uuid.UUID,
        target_user_id: uuid.UUID | None,
        target_email: str | None,
        role: str,
    ) -> MembershipRow:
        rbac.require(await self._actor_roles(actor_id), str(org_id), Permission.MEMBER_INVITE)
        if role not in set(Role):
            raise AppError(ErrorCode.INVALID_PAYLOAD, f"invalid role '{role}'")
        target = None
        if target_user_id is not None:
            target = await self._repo.get_user(target_user_id)
        elif target_email is not None:
            target = await self._repo.get_user_by_email(target_email)
        if target is None:
            raise AppError(ErrorCode.MEMBER_NOT_FOUND, "user to add not found")
        if await self._repo.get_membership(org_id, target.user_id) is not None:
            raise AppError(ErrorCode.MEMBER_EXISTS, "user is already a member")
        membership = MembershipRow(org_id=org_id, user_id=target.user_id, role=role)
        await self._repo.insert_membership(membership)
        await self._repo.add_event(
            "org",
            org_id,
            events.MEMBER_ADDED,
            {"org_id": str(org_id), "user_id": str(target.user_id), "role": role},
        )
        await self._commit()
        await self._publisher.publish(
            events.MEMBER_ADDED,
            {"org_id": str(org_id), "user_id": str(target.user_id), "role": role},
        )
        return membership

    async def list_members(self, *, actor_id: uuid.UUID, org_id: uuid.UUID) -> list[MembershipRow]:
        rbac.require(await self._actor_roles(actor_id), str(org_id), Permission.MEMBER_READ)
        return await self._repo.list_members(org_id)

    async def remove_member(
        self, *, actor_id: uuid.UUID, org_id: uuid.UUID, target_user_id: uuid.UUID
    ) -> None:
        rbac.require(await self._actor_roles(actor_id), str(org_id), Permission.MEMBER_REMOVE)
        membership = await self._repo.get_membership(org_id, target_user_id)
        if membership is None:
            raise AppError(ErrorCode.MEMBER_NOT_FOUND, "membership not found")
        if membership.role == Role.OWNER and await self._repo.count_owners(org_id) <= 1:
            raise AppError(ErrorCode.CANNOT_REMOVE_LAST_OWNER, "cannot remove the last owner")
        await self._repo.delete_membership(membership)
        await self._repo.add_event(
            "org",
            org_id,
            events.MEMBER_REMOVED,
            {"org_id": str(org_id), "user_id": str(target_user_id)},
        )
        await self._commit()
        await self._publisher.publish(
            events.MEMBER_REMOVED, {"org_id": str(org_id), "user_id": str(target_user_id)}
        )
