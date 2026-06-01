"""Domain value objects (enums). Pure, no I/O."""

from __future__ import annotations

from enum import IntEnum, StrEnum


class KycTier(IntEnum):
    """KYC assurance tiers. 0 = none, 1 = basic, 2 = enhanced. Requestable tiers are {1, 2}."""

    NONE = 0
    BASIC = 1
    ENHANCED = 2


class FlagKind(StrEnum):
    SANCTIONS = "sanctions"  # reserved for Phase 2 (unused in the MVP)
    VELOCITY = "velocity"
    GEO = "geo"
    MANUAL = "manual"


class FlagSeverity(StrEnum):
    INFO = "info"
    WARN = "warn"
    BLOCK = "block"
