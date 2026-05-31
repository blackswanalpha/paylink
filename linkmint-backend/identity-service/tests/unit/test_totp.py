from __future__ import annotations

import pyotp

from app.security.totp import generate_totp_secret, provisioning_uri, verify_totp


def test_valid_code_accepts_wrong_rejects() -> None:
    secret = generate_totp_secret()
    code = pyotp.TOTP(secret).now()
    assert verify_totp(secret, code)
    wrong = "000000" if code != "000000" else "111111"
    assert not verify_totp(secret, wrong)
    assert not verify_totp(secret, "")


def test_provisioning_uri() -> None:
    uri = provisioning_uri("JBSWY3DPEHPK3PXP", account_name="a@b.com", issuer="linkmint-identity")
    assert uri.startswith("otpauth://totp/")
    assert "issuer=linkmint-identity" in uri
