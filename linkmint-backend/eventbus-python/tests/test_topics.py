from __future__ import annotations

import pytest

from linkmint_eventbus.config import EventbusSettings
from linkmint_eventbus.topics import DOMAINS, topic_for


@pytest.mark.parametrize(
    ("name", "want"),
    [
        ("paylink.verified", "paylink"),
        ("chain.paylink.verified", "chain"),
        ("payment.proof_received", "payment"),
        ("merchant.bank_account.added", "merchant"),
        ("identity.user.registered", "identity"),
        ("singleton", "singleton"),
    ],
)
def test_topic_for(name: str, want: str) -> None:
    assert topic_for(name) == want


def test_domains_map_to_themselves() -> None:
    for d in DOMAINS:
        assert topic_for(f"{d}.some.event") == d


def test_settings_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("KAFKA_BROKERS", "a:9092, b:9092 ,")
    monkeypatch.setenv("KAFKA_CLIENT_ID", "svc")
    monkeypatch.setenv("KAFKA_CONSUMER_GROUP", "grp")
    s = EventbusSettings()
    assert s.broker_list == ["a:9092", "b:9092"]
    assert s.client_id == "svc"
    assert s.consumer_group == "grp"


def test_settings_defaults(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("KAFKA_BROKERS", raising=False)
    monkeypatch.delenv("KAFKA_CLIENT_ID", raising=False)
    s = EventbusSettings()
    assert s.broker_list == []
    assert s.client_id == "linkmint"
