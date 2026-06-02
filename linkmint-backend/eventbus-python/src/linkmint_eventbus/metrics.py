"""Prometheus counter for events consumed off the bus (work18).

Defined on the default registry, so it appears automatically on each Python consumer service's
existing ``/metrics`` mount. Labels are PII-free: the event NAME and the handle result.
"""

from __future__ import annotations

from prometheus_client import Counter

BUS_MESSAGES_CONSUMED = Counter(
    "bus_messages_consumed_total",
    "Domain events consumed from the bus, by event name and handle result.",
    ["event", "result"],
)
