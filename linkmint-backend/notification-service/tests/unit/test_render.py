"""Template rendering — safe ``$placeholder`` substitution."""

from __future__ import annotations

from app.templating.render import render


def test_substitutes_present_keys() -> None:
    out = render(
        "$amount $currency to $paylink_id",
        {"amount": "1500", "currency": "KES", "paylink_id": "pl_1"},
    )
    assert out == "1500 KES to pl_1"


def test_missing_key_left_literal_not_raised() -> None:
    assert render("hi $name", {}) == "hi $name"


def test_non_str_values_coerced() -> None:
    assert render("n=$n ok=$ok", {"n": 7, "ok": True}) == "n=7 ok=True"


def test_no_code_execution() -> None:
    # ``$`` templating is not f-strings: dunder access stays literal, never evaluated.
    assert render("${__import__}", {}) == "${__import__}"
