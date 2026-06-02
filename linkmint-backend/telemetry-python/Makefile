# telemetry-python (work18). Shared OpenTelemetry helper; mirrors the eventbus-python recipe.

.PHONY: install lint test cover

install:
	pip install -e ".[dev]"

lint:
	ruff check .
	black --check .
	mypy src

test:
	pytest

# The DoD coverage gate (>=80%) is enforced by pytest addopts (--cov-fail-under=80).
cover:
	pytest
