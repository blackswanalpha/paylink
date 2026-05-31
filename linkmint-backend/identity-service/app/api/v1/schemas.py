"""Request/response models for the identity API.

No secrets are ever returned except the one-time ``full_key`` at API-key issuance. Password, MFA
secret, refresh-token, and API-key hashes never appear in any response.
"""

from __future__ import annotations

import re
from datetime import datetime

from pydantic import BaseModel, Field, field_validator, model_validator

from app.domain.models import OrgType, Role, Scope

_EMAIL_RE = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")


def _norm_email(value: str | None) -> str | None:
    if value is None:
        return None
    v = value.strip().lower()
    if not _EMAIL_RE.match(v):
        raise ValueError("invalid email address")
    return v


# ── auth ──
class RegisterRequest(BaseModel):
    email: str | None = None
    phone: str | None = None
    password: str = Field(min_length=8, max_length=256)

    _v_email = field_validator("email")(classmethod(lambda cls, v: _norm_email(v)))

    @model_validator(mode="after")
    def _require_identifier(self) -> RegisterRequest:
        if not self.email and not self.phone:
            raise ValueError("one of email or phone is required")
        return self


class RegisterResponse(BaseModel):
    user_id: str
    status: str


class LoginRequest(BaseModel):
    email: str | None = None
    phone: str | None = None
    password: str
    mfa_code: str | None = None

    _v_email = field_validator("email")(classmethod(lambda cls, v: _norm_email(v)))

    @model_validator(mode="after")
    def _exactly_one_identifier(self) -> LoginRequest:
        if bool(self.email) == bool(self.phone):
            raise ValueError("provide exactly one of email or phone")
        return self

    @property
    def identifier(self) -> str:
        return self.email or self.phone or ""


class TokenResponse(BaseModel):
    access_token: str
    refresh_token: str
    token_type: str = "Bearer"
    expires_in: int


class RefreshRequest(BaseModel):
    refresh_token: str = Field(min_length=1)


class LogoutRequest(BaseModel):
    refresh_token: str = Field(min_length=1)


class OAuthStartRequest(BaseModel):
    redirect_uri: str | None = None
    state: str | None = None


class OAuthStartResponse(BaseModel):
    authorize_url: str
    state: str


class OAuthCallbackRequest(BaseModel):
    code: str = Field(min_length=1)
    state: str = ""
    redirect_uri: str | None = None


class MfaEnrollResponse(BaseModel):
    secret: str
    otpauth_uri: str


class MfaCodeRequest(BaseModel):
    code: str = Field(min_length=1, max_length=16)


class MfaVerifyResponse(BaseModel):
    enabled: bool = True


# ── users / api keys ──
class OrgRoleEntry(BaseModel):
    org_id: str
    role: str


class UserProfileResponse(BaseModel):
    user_id: str
    email: str | None
    phone: str | None
    kyc_tier: int
    status: str
    roles: list[OrgRoleEntry]
    user_roles: list[str]
    created_at: datetime
    last_login_at: datetime | None


class UpdateProfileRequest(BaseModel):
    email: str | None = None
    phone: str | None = None

    _v_email = field_validator("email")(classmethod(lambda cls, v: _norm_email(v)))


class IssueApiKeyRequest(BaseModel):
    org_id: str
    name: str = Field(min_length=1, max_length=128)
    scopes: list[Scope] = Field(default_factory=list)


class IssueApiKeyResponse(BaseModel):
    api_key_id: str
    org_id: str
    name: str
    prefix: str
    # The full secret key, shown EXACTLY once at issuance. Null on an idempotent replay — the secret
    # is never cached/persisted; if the original response was lost, revoke this key and re-issue.
    full_key: str | None
    scopes: list[str]
    status: str
    created_at: datetime


class ApiKeyResponse(BaseModel):
    api_key_id: str
    org_id: str
    name: str
    prefix: str
    scopes: list[str]
    status: str
    created_at: datetime
    revoked_at: datetime | None


class ApiKeyListResponse(BaseModel):
    items: list[ApiKeyResponse]


class RevokeApiKeyResponse(BaseModel):
    api_key_id: str
    status: str


# ── internal admin (read-only; consumed by admin-backoffice over the trusted internal surface) ──
class AdminUserView(BaseModel):
    user_id: str
    email: str | None
    phone: str | None
    kyc_tier: int
    status: str
    created_at: datetime
    last_login_at: datetime | None


class AdminUserListResponse(BaseModel):
    items: list[AdminUserView]


# ── organizations / members ──
class CreateOrgRequest(BaseModel):
    name: str = Field(min_length=1, max_length=200)
    type: OrgType


class OrgResponse(BaseModel):
    org_id: str
    name: str
    type: str
    role: str
    created_at: datetime


class AddMemberRequest(BaseModel):
    user_id: str | None = None
    email: str | None = None
    role: Role

    _v_email = field_validator("email")(classmethod(lambda cls, v: _norm_email(v)))

    @model_validator(mode="after")
    def _exactly_one_target(self) -> AddMemberRequest:
        if bool(self.user_id) == bool(self.email):
            raise ValueError("provide exactly one of user_id or email")
        return self


class MemberResponse(BaseModel):
    org_id: str
    user_id: str
    role: str


class MemberListResponse(BaseModel):
    items: list[MemberResponse]


# ── sessions ──
class SessionResponse(BaseModel):
    session_id: str
    user_agent: str | None
    ip: str | None
    created_at: datetime
    expires_at: datetime
    current: bool


class SessionListResponse(BaseModel):
    items: list[SessionResponse]
