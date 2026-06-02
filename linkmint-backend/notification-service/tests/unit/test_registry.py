"""Template registry — per-channel resolution, locale fallback, inactive skipping, subjects."""

from __future__ import annotations

import pytest

from app.db.models import TemplateRow
from app.domain.models import Channel
from app.templating.registry import TemplateRegistry
from tests._support import FakeRepository, default_templates


@pytest.fixture
def registry() -> TemplateRegistry:
    return TemplateRegistry(FakeRepository())  # type: ignore[arg-type]


async def test_resolves_both_channels_for_event(registry: TemplateRegistry) -> None:
    out = await registry.resolve_for_event("paylink.verified", "en")
    assert set(out.keys()) == {Channel.SMS, Channel.EMAIL}
    assert out[Channel.SMS].template_id == "sms.paylink_verified.en"


async def test_locale_falls_back_to_en(registry: TemplateRegistry) -> None:
    out = await registry.resolve_for_event("paylink.verified", "fr")
    assert set(out.keys()) == {Channel.SMS, Channel.EMAIL}  # only en seeded → fallback


async def test_unknown_event_resolves_to_empty(registry: TemplateRegistry) -> None:
    assert await registry.resolve_for_event("nope.event", "en") == {}


async def test_inactive_template_skipped() -> None:
    templates = default_templates()
    # Deactivate the SMS paylink_verified template.
    for t in templates:
        if t.template_id == "sms.paylink_verified.en":
            t.active = False
    registry = TemplateRegistry(FakeRepository(templates=templates))  # type: ignore[arg-type]
    out = await registry.resolve_for_event("paylink.verified", "en")
    assert set(out.keys()) == {Channel.EMAIL}


async def test_prefers_exact_locale_over_en() -> None:
    templates = default_templates()
    templates.append(
        TemplateRow(
            template_id="sms.paylink_verified.fr",
            channel="sms",
            locale="fr",
            body="FR $amount",
            version=1,
            active=True,
        )
    )
    registry = TemplateRegistry(FakeRepository(templates=templates))  # type: ignore[arg-type]
    out = await registry.resolve_for_event("paylink.verified", "fr")
    assert out[Channel.SMS].locale == "fr"


def test_subject_for_known_and_unknown() -> None:
    assert TemplateRegistry.subject_for("paylink.verified", {}) == "Your PayLink is verified"
    assert TemplateRegistry.subject_for("mystery.event", {}) == "LinkMint notification"
