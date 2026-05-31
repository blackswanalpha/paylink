from __future__ import annotations

from app.security.refresh_tokens import (
    hash_refresh_token,
    mint_refresh_token,
    refresh_token_matches,
)


def test_mint_unique() -> None:
    assert mint_refresh_token() != mint_refresh_token()


def test_hash_stable_and_hides_token() -> None:
    token = mint_refresh_token()
    assert hash_refresh_token(token) == hash_refresh_token(token)
    assert token not in hash_refresh_token(token)


def test_matches_constant_time() -> None:
    token = mint_refresh_token()
    assert refresh_token_matches(token, hash_refresh_token(token))
    assert not refresh_token_matches("some-other-token", hash_refresh_token(token))
