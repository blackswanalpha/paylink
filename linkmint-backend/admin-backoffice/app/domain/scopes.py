"""Default-deny scope resolution.

A staff member's effective scopes are the union of their ``admin.staff`` grant (the production
source of truth) and the local-dev env seed (``ADMIN_DEV_STAFF_GRANTS``). A ``superuser`` grant
expands to every scope. An unknown ``sub`` resolves to the empty set — locked out by default.
"""

from __future__ import annotations

from app.config import Settings
from app.db.repositories import AdminRepository
from app.domain.models import SCOPES


class ScopeResolver:
    def __init__(self, repo: AdminRepository, settings: Settings) -> None:
        self._repo = repo
        self._settings = settings

    async def resolve(self, sub: str) -> frozenset[str]:
        granted: set[str] = set(self._settings.dev_staff_grant_map().get(sub, set()))
        staff = await self._repo.get_staff(sub)
        if staff is not None:
            granted.update(staff.scopes)
        if "superuser" in granted:
            return SCOPES
        return frozenset(granted)
