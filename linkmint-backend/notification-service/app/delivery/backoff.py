"""Retry backoff schedule (spec §2.7: ``30s, 2m, 10m, 1h, 6h`` — max 5 retries).

Semantics: ``attempts_done`` is the number of send attempts already made (including the one that
just failed). The initial send is attempt 1; on failure it schedules retry #1 after 30s, etc. After
the 5 backoffs are exhausted (``attempts_done`` reaches 6), there is no further retry.

The five documented countdowns sum to ~7h26m; the spec's "~24h" is a loose ceiling, not a literal
total — pin the schedule to the verbatim ``30s/2m/10m/1h/6h``.
"""

from __future__ import annotations

BACKOFF_SCHEDULE_SECONDS: tuple[int, ...] = (30, 120, 600, 3600, 21600)
MAX_RETRIES = len(BACKOFF_SCHEDULE_SECONDS)  # 5


def next_countdown(attempts_done: int) -> int | None:
    """Seconds until the next retry, or ``None`` when retries are exhausted."""
    if 1 <= attempts_done <= MAX_RETRIES:
        return BACKOFF_SCHEDULE_SECONDS[attempts_done - 1]
    return None
