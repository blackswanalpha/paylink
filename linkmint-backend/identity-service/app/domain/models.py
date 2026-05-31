"""Domain value objects (enums) + small result types. Pure, no I/O."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class UserStatus(StrEnum):
    ACTIVE = "ACTIVE"
    SUSPENDED = "SUSPENDED"
    DELETED = "DELETED"


class OrgType(StrEnum):
    MERCHANT = "merchant"
    DEVELOPER = "developer"
    ADMIN = "admin"


class Role(StrEnum):
    """Org-scoped roles, highest privilege first."""

    OWNER = "owner"
    ADMIN = "admin"
    DEVELOPER = "developer"
    OPERATOR = "operator"
    VIEWER = "viewer"


class UserRole(StrEnum):
    """User-level (non-org) roles."""

    PAYER = "payer"


class ApiKeyStatus(StrEnum):
    ACTIVE = "ACTIVE"
    REVOKED = "REVOKED"


class MfaKind(StrEnum):
    TOTP = "totp"
    WEBAUTHN = "webauthn"  # Phase 2
    SMS_OTP = "sms_otp"  # Phase 2


class Permission(StrEnum):
    ORG_READ = "org:read"
    ORG_UPDATE = "org:update"
    ORG_DELETE = "org:delete"
    MEMBER_READ = "member:read"
    MEMBER_INVITE = "member:invite"
    MEMBER_REMOVE = "member:remove"
    APIKEY_READ = "apikey:read"
    APIKEY_ISSUE = "apikey:issue"
    APIKEY_REVOKE = "apikey:revoke"


class Scope(StrEnum):
    """API-key scopes — the downstream (gateway-facing) surface, a subset granted per role."""

    PAYLINKS_READ = "paylinks:read"
    PAYLINKS_WRITE = "paylinks:write"
    PAYMENTS_READ = "payments:read"
    PAYMENTS_WRITE = "payments:write"


@dataclass(frozen=True)
class AuthTokens:
    access_token: str
    refresh_token: str
    expires_in: int
    token_type: str = "Bearer"
