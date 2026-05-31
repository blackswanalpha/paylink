"""Wire the per-request service bundle from shared singletons + a session-bound repository.

``get_services`` (deps.py) builds a :class:`ServiceDeps` per request and yields :class:`Services`;
tests build the same bundle over in-memory fakes, so there is a single override point.
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from app.config import Settings
from app.db.repositories import IdentityRepository
from app.domain.api_keys_service import ApiKeysService
from app.domain.auth_service import AuthService
from app.domain.mfa_service import MfaService
from app.domain.orgs_service import OrgsService
from app.domain.sessions_service import SessionsService
from app.domain.users_service import UsersService
from app.events.publisher import Publisher
from app.security.jwt import JwtIssuer
from app.security.login_attempts import FailedLoginCounter
from app.security.mfa_crypto import MfaCipher
from app.security.oauth.registry import OAuthResolver
from app.security.passwords import PasswordHashing

_Commit = Callable[[], Awaitable[None]]


@dataclass(frozen=True)
class ServiceDeps:
    repo: IdentityRepository
    commit: _Commit
    settings: Settings
    publisher: Publisher
    passwords: PasswordHashing
    jwt: JwtIssuer
    mfa_cipher: MfaCipher
    oauth: OAuthResolver
    failed_login: FailedLoginCounter


@dataclass(frozen=True)
class Services:
    auth: AuthService
    users: UsersService
    orgs: OrgsService
    api_keys: ApiKeysService
    sessions: SessionsService
    mfa: MfaService


def build_services(d: ServiceDeps) -> Services:
    sessions = SessionsService(d.repo, d.commit, d.jwt, d.publisher, d.settings)
    mfa = MfaService(d.repo, d.commit, d.mfa_cipher, d.publisher, d.settings)
    auth = AuthService(
        d.repo,
        d.commit,
        d.passwords,
        d.publisher,
        d.settings,
        sessions,
        mfa,
        d.oauth,
        d.failed_login,
    )
    users = UsersService(d.repo, d.commit, d.publisher)
    orgs = OrgsService(d.repo, d.commit, d.publisher)
    api_keys = ApiKeysService(d.repo, d.commit, d.passwords, d.publisher)
    return Services(
        auth=auth, users=users, orgs=orgs, api_keys=api_keys, sessions=sessions, mfa=mfa
    )
