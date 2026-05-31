"""Authentication: register, login (password + optional TOTP), refresh, logout, OAuth callback.

Delegates session/token mechanics to :class:`SessionsService` and MFA checks to :class:`MfaService`
so this stays focused on credential verification + account resolution.
"""

from __future__ import annotations

import secrets
import uuid
from collections.abc import Awaitable, Callable
from datetime import UTC, datetime

from app.config import Settings
from app.db.models import OAuthIdentityRow, UserRow
from app.db.repositories import IdentityRepository
from app.domain.mfa_service import MfaService
from app.domain.models import AuthTokens, UserStatus
from app.domain.sessions_service import SessionsService
from app.errors import AppError, ErrorCode
from app.events import publisher as events
from app.events.publisher import Publisher
from app.security.login_attempts import FailedLoginCounter
from app.security.oauth.provider import AuthorizeRequest, OAuthError
from app.security.oauth.registry import OAuthResolver
from app.security.passwords import PasswordHashing

_Commit = Callable[[], Awaitable[None]]


class AuthService:
    def __init__(
        self,
        repo: IdentityRepository,
        commit: _Commit,
        passwords: PasswordHashing,
        publisher: Publisher,
        settings: Settings,
        sessions: SessionsService,
        mfa: MfaService,
        oauth: OAuthResolver,
        failed_login: FailedLoginCounter,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._passwords = passwords
        self._publisher = publisher
        self._settings = settings
        self._sessions = sessions
        self._mfa = mfa
        self._oauth = oauth
        self._failed_login = failed_login

    async def register(self, *, email: str | None, phone: str | None, password: str) -> UserRow:
        if email and await self._repo.get_user_by_email(email):
            raise AppError(ErrorCode.EMAIL_TAKEN, "email already registered")
        if phone and await self._repo.get_user_by_phone(phone):
            raise AppError(ErrorCode.PHONE_TAKEN, "phone already registered")
        user = UserRow(
            user_id=uuid.uuid4(),
            email=email,
            phone=phone,
            password_hash=self._passwords.hash(password),
            status=UserStatus.ACTIVE.value,
        )
        await self._repo.insert_user(user)
        await self._repo.add_event(
            "user", user.user_id, events.USER_REGISTERED, {"user_id": str(user.user_id)}
        )
        await self._commit()
        await self._publisher.publish(events.USER_REGISTERED, {"user_id": str(user.user_id)})
        return user

    async def login(
        self,
        *,
        identifier: str,
        password: str,
        mfa_code: str | None,
        user_agent: str | None,
        ip: str | None,
    ) -> AuthTokens:
        user = await self._lookup(identifier)
        if (
            user is None
            or user.password_hash is None
            or not self._passwords.verify(user.password_hash, password)
        ):
            await self._record_failure(identifier)
            raise AppError(ErrorCode.INVALID_CREDENTIALS, "invalid credentials")
        if user.status != UserStatus.ACTIVE:
            raise AppError(ErrorCode.USER_SUSPENDED, "account is not active")
        if await self._mfa.is_required(user.user_id):
            if not mfa_code:
                raise AppError(ErrorCode.MFA_REQUIRED, "MFA code required")
            if not await self._mfa.verify_login(user.user_id, mfa_code):
                await self._record_failure(identifier)
                raise AppError(ErrorCode.MFA_INVALID, "invalid MFA code")
        await self._failed_login.reset(identifier)
        user.last_login_at = datetime.now(UTC)
        return await self._sessions.issue(user, user_agent=user_agent, ip=ip)

    async def refresh(
        self, refresh_token: str, *, user_agent: str | None, ip: str | None
    ) -> AuthTokens:
        return await self._sessions.rotate(refresh_token, user_agent=user_agent, ip=ip)

    async def logout(self, user_id: uuid.UUID, refresh_token: str) -> None:
        await self._sessions.logout(user_id, refresh_token)

    def oauth_start(
        self, provider: str, *, state: str | None, redirect_uri: str | None
    ) -> AuthorizeRequest:
        prov = self._oauth.get(provider)
        if prov is None:
            raise AppError(ErrorCode.OAUTH_PROVIDER_UNKNOWN, f"unknown provider '{provider}'")
        return prov.authorize(state=state or secrets.token_urlsafe(16), redirect_uri=redirect_uri)

    async def oauth_callback(
        self,
        provider: str,
        *,
        code: str,
        state: str,
        redirect_uri: str | None,
        user_agent: str | None,
        ip: str | None,
    ) -> AuthTokens:
        prov = self._oauth.get(provider)
        if prov is None:
            raise AppError(ErrorCode.OAUTH_PROVIDER_UNKNOWN, f"unknown provider '{provider}'")
        try:
            identity = await prov.exchange_code(code=code, state=state, redirect_uri=redirect_uri)
        except OAuthError as exc:
            raise AppError(ErrorCode.OAUTH_EXCHANGE_FAILED, "OAuth code exchange failed") from exc

        newly_registered = False
        link = await self._repo.get_oauth_identity(identity.provider, identity.subject)
        if link is not None:
            user = await self._repo.get_user(link.user_id)
            if user is None:  # dangling link — treat as invalid
                raise AppError(ErrorCode.OAUTH_EXCHANGE_FAILED, "linked account not found")
        else:
            # SECURITY: never auto-merge into an existing account by email. A provider may return an
            # attacker-controlled / unverified email, so matching `get_user_by_email` here would be
            # an account-takeover vector. We always create a *fresh* OAuth-only account keyed on
            # (provider, subject). The email is attached only if it isn't already taken (otherwise
            # left NULL); claiming/linking an existing account is a deliberate authenticated flow
            # (follow-up: verified-email link).
            email = identity.email
            if email is not None and await self._repo.get_user_by_email(email) is not None:
                email = None
            user = UserRow(
                user_id=uuid.uuid4(),
                email=email,
                phone=None,
                password_hash=None,  # OAuth-only account
                status=UserStatus.ACTIVE.value,
            )
            await self._repo.insert_user(user)
            await self._repo.add_event(
                "user", user.user_id, events.USER_REGISTERED, {"user_id": str(user.user_id)}
            )
            newly_registered = True
            await self._repo.insert_oauth_identity(
                OAuthIdentityRow(
                    provider=identity.provider, subject=identity.subject, user_id=user.user_id
                )
            )

        if user.status != UserStatus.ACTIVE:
            raise AppError(ErrorCode.USER_SUSPENDED, "account is not active")
        user.last_login_at = datetime.now(UTC)
        tokens = await self._sessions.issue(user, user_agent=user_agent, ip=ip)  # commits writes
        if newly_registered:
            await self._publisher.publish(events.USER_REGISTERED, {"user_id": str(user.user_id)})
        return tokens

    async def _lookup(self, identifier: str) -> UserRow | None:
        if "@" in identifier:
            return await self._repo.get_user_by_email(identifier)
        return await self._repo.get_user_by_phone(identifier)

    async def _record_failure(self, identifier: str) -> None:
        count = await self._failed_login.record(identifier)
        if count >= self._settings.auth_failed_threshold:
            await self._publisher.publish(
                events.AUTH_FAILED, {"identifier": identifier, "failures": count}
            )
