"""End-to-end publish→consume over a real broker.

Opt-in: set EVENTBUS_INTEGRATION=1 and KAFKA_BROKERS=<host:port> (e.g. the docker-compose Redpanda
at localhost:19092) to run. The default `pytest` run is unit-only — the cross-language wire contract
is fully proven by the golden-byte tests, which need no broker, and eventbus-go's testcontainer
proves the transport self-contained. This adds a live Python publisher↔consumer round-trip.
"""

from __future__ import annotations

import asyncio
import os

import pytest

BROKER = os.getenv("KAFKA_BROKERS", "")

pytestmark = [
    pytest.mark.integration,
    pytest.mark.skipif(
        os.getenv("EVENTBUS_INTEGRATION") != "1" or not BROKER,
        reason="set EVENTBUS_INTEGRATION=1 and KAFKA_BROKERS=<host:port> to run",
    ),
]


async def _create_topic(broker: str, topic: str) -> None:
    from aiokafka.admin import AIOKafkaAdminClient, NewTopic

    admin = AIOKafkaAdminClient(bootstrap_servers=broker)
    await admin.start()
    try:
        await admin.create_topics([NewTopic(topic, num_partitions=1, replication_factor=1)])
    except Exception:  # noqa: BLE001 — already exists is fine
        pass
    finally:
        await admin.close()


async def test_publish_consume_roundtrip() -> None:
    from linkmint_eventbus import KafkaConsumer, KafkaPublisher

    brokers = [b.strip() for b in BROKER.split(",") if b.strip()]
    await _create_topic(BROKER, "paylink")

    pub = KafkaPublisher(brokers, "test-suite")
    await pub.start()
    await pub.publish("paylink.verified", "PLK_1", {"pl_id": "PLK_1", "amount": "500"})
    await pub.stop()

    con = KafkaConsumer(brokers, ["paylink"], "py-rt-group")
    got: asyncio.Queue[tuple[str, dict]] = asyncio.Queue()

    async def handle(name: str, payload: dict) -> None:
        await got.put((name, payload))

    task = asyncio.create_task(con.run(handle))
    try:
        name, payload = await asyncio.wait_for(got.get(), timeout=45)
        assert name == "paylink.verified"
        assert payload == {"amount": "500", "pl_id": "PLK_1"}
    finally:
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task
