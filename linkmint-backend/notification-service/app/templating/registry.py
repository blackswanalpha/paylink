"""Template registry — resolves the active template per (event, channel, locale).

A single DB read per intake (``list_active_templates_for_event``) is grouped in memory: for each
Phase-1 channel (SMS, EMAIL) the locale match is preferred, else the ``en`` fallback. Email subjects
are a small in-code map (rendered with the same ``$placeholder`` substitution) — keeps the seeded
template bodies single-purpose.
"""

from __future__ import annotations

from app.db.models import TemplateRow
from app.db.repository import NotifyRepository
from app.domain.models import Channel
from app.templating.render import render

DEFAULT_LOCALE = "en"

# Per-event email subjects (placeholders allowed; rendered with the event ``data``).
EMAIL_SUBJECTS: dict[str, str] = {
    "paylink.verified": "Your PayLink is verified",
    "payment.failed": "Your payment could not be completed",
}


class TemplateRegistry:
    def __init__(self, repo: NotifyRepository) -> None:
        self._repo = repo

    async def resolve_for_event(
        self, event_kind: str, locale: str | None
    ) -> dict[Channel, TemplateRow]:
        """Active template per Phase-1 channel for this event, with locale→``en`` fallback."""
        rows = await self._repo.list_active_templates_for_event(event_kind)
        by_channel: dict[str, dict[str, TemplateRow]] = {}
        for row in rows:
            by_channel.setdefault(row.channel, {})[row.locale] = row

        out: dict[Channel, TemplateRow] = {}
        for channel in (Channel.SMS, Channel.EMAIL):
            locales = by_channel.get(channel.value)
            if not locales:
                continue
            chosen = locales.get(locale or DEFAULT_LOCALE) or locales.get(DEFAULT_LOCALE)
            if chosen is not None:
                out[channel] = chosen
        return out

    @staticmethod
    def subject_for(event_kind: str, data: dict[str, object]) -> str:
        return render(EMAIL_SUBJECTS.get(event_kind, "LinkMint notification"), data)
