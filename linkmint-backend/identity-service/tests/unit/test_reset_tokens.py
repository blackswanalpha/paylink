from __future__ import annotations

from app.security.reset_tokens import (
    hash_reset_token,
    mint_reset_token,
    reset_token_matches,
)


def test_mint_unique() -> None:
    assert mint_reset_token() != mint_reset_token()


def test_hash_stable_and_hides_token() -> None:
    token = mint_reset_token()
    assert hash_reset_token(token) == hash_reset_token(token)
    assert token not in hash_reset_token(token)


def test_matches_constant_time() -> None:
    token = mint_reset_token()
    assert reset_token_matches(token, hash_reset_token(token))
    assert not reset_token_matches("some-other-token", hash_reset_token(token))
