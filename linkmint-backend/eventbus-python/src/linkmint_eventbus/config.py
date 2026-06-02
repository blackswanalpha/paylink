"""Event-bus settings, sourced from the same ``KAFKA_*`` env vars the Go lib reads."""

from __future__ import annotations

from pydantic_settings import BaseSettings, SettingsConfigDict


class EventbusSettings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="KAFKA_", extra="ignore")

    brokers: str = ""  # comma-separated host:port list (KAFKA_BROKERS)
    client_id: str = "linkmint"  # KAFKA_CLIENT_ID
    consumer_group: str = ""  # KAFKA_CONSUMER_GROUP (consumer only)

    @property
    def broker_list(self) -> list[str]:
        return [b.strip() for b in self.brokers.split(",") if b.strip()]
