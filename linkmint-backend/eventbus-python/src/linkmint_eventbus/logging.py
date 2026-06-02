"""A structlog logger accessor for the lib. The consuming service owns structlog configuration; this
just binds a named logger (structlog works with sane defaults even if unconfigured)."""

from __future__ import annotations

from typing import Any

import structlog


def get_logger(name: str = "eventbus") -> Any:
    return structlog.get_logger(name)
