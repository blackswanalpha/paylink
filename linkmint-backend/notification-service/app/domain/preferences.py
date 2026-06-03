"""Notification preferences — the canonical channel/event sets, defaults, and the gate used by the
fan-out (:mod:`app.domain.service`) to decide whether a given (event, channel) is delivered.

Opt-out model: anything not explicitly turned off is on. A recipient with no stored row gets
:func:`default_preferences` (everything enabled); within a stored map, an absent key is also treated
as enabled, so adding a new channel/event never silently mutes existing recipients.

The event list mirrors ``app.events.consumer.KNOWN_EVENTS`` but is declared here independently to
avoid an import cycle (consumer → service → preferences). Keep the two in sync.
"""

from __future__ import annotations

from dataclasses import dataclass

# In-app inbox is the address-scoped notification center; email/sms are the templated fan-out.
PREF_CHANNELS: tuple[str, ...] = ("in_app", "email", "sms")
# Mirrors app.events.consumer.KNOWN_EVENTS (kept literal + ordered for a stable UI).
PREF_EVENTS: tuple[str, ...] = (
    "paylink.created",
    "paylink.verified",
    "paylink.cancelled",
    "payment.failed",
)


@dataclass(frozen=True)
class Preferences:
    """A recipient's effective preferences. Maps may be partial — a missing key means enabled."""

    channels: dict[str, bool]
    events: dict[str, bool]

    def allows(self, event_kind: str, channel: str) -> bool:
        """True iff this (event, channel) should be delivered (opt-out: unknown keys = enabled)."""
        return bool(self.events.get(event_kind, True)) and bool(self.channels.get(channel, True))


def default_preferences() -> Preferences:
    """Everything enabled — what a recipient with no stored row gets."""
    return Preferences(
        channels=dict.fromkeys(PREF_CHANNELS, True),
        events=dict.fromkeys(PREF_EVENTS, True),
    )


def _clean(values: dict[str, object] | None, allowed: tuple[str, ...]) -> dict[str, bool]:
    """Keep only known keys, coercing values to bool (drops anything unrecognized/garbage)."""
    if not values:
        return {}
    return {k: bool(v) for k, v in values.items() if k in allowed}


def merge_preferences(
    channels: dict[str, object] | None,
    events: dict[str, object] | None,
) -> Preferences:
    """Overlay stored/partial maps onto the all-enabled defaults, ignoring unknown keys.

    Used both to render the API view (stored row → full effective set) and to apply a PUT patch.
    """
    base = default_preferences()
    return Preferences(
        channels={**base.channels, **_clean(channels, PREF_CHANNELS)},
        events={**base.events, **_clean(events, PREF_EVENTS)},
    )
