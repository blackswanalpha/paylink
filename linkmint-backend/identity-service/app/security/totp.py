"""TOTP MFA primitives (pyotp). Phase 1 supports the ``totp`` factor kind."""

from __future__ import annotations

import pyotp


def generate_totp_secret() -> str:
    return pyotp.random_base32()


def provisioning_uri(secret: str, *, account_name: str, issuer: str) -> str:
    return pyotp.TOTP(secret).provisioning_uri(name=account_name, issuer_name=issuer)


def verify_totp(secret: str, code: str, *, valid_window: int = 1) -> bool:
    if not code:
        return False
    return pyotp.TOTP(secret).verify(code, valid_window=valid_window)
