"""Template rendering — safe ``$placeholder`` substitution from event ``data``.

Uses :class:`string.Template` ``safe_substitute``: no ``eval``/format-string injection surface, and
missing keys are left as the literal ``$placeholder`` rather than raising — so a partially-populated
event still renders. Values are coerced to ``str``.
"""

from __future__ import annotations

from collections.abc import Mapping
from string import Template
from typing import Any


def render(body: str, data: Mapping[str, Any]) -> str:
    coerced = {key: str(value) for key, value in data.items()}
    return Template(body).safe_substitute(coerced)
