"""linkmint_eventbus — LinkMint's shared Kafka client library (work15).

A canonical JSON Envelope plus an aiokafka publisher and a commit-after-handle consumer-group
consumer. Serializes byte-identically to the Go library (eventbus-go), so either language produces
events the other consumes. Transport is Kafka via Redpanda (ADR-011); the logical event model is
``workload/catalog.md``. Delivery is at-least-once — handlers MUST be idempotent. Payloads carry
ids/metadata only, never secrets.
"""

from __future__ import annotations

from .config import EventbusSettings
from .consumer import HandleFunc, KafkaConsumer
from .envelope import Envelope
from .publisher import KafkaPublisher
from .topics import DOMAINS, topic_for

__all__ = [
    "Envelope",
    "KafkaPublisher",
    "KafkaConsumer",
    "HandleFunc",
    "EventbusSettings",
    "topic_for",
    "DOMAINS",
]
