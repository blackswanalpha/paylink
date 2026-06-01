"""Document object-store seam.

Uploaded document *bytes* live in an object store; the DB only stores the ``s3_key`` that addresses
them. ``LocalObjectStore`` (filesystem) is the dev/test default; ``S3ObjectStore`` (lazy ``boto3``)
is the production seam (``MERCHANT_S3_*``) and is NOT exercised locally. Unit tests use an in-memory
fake with this same ABC.
"""

from __future__ import annotations

import os
from abc import ABC, abstractmethod
from pathlib import Path
from typing import Any

from app.config import Settings


class ObjectStore(ABC):
    @abstractmethod
    def put(self, key: str, data: bytes, *, content_type: str | None = None) -> None:
        """Store ``data`` under ``key`` (overwrites)."""

    @abstractmethod
    def get(self, key: str) -> bytes:
        """Fetch the bytes stored under ``key`` (raises if absent)."""


class LocalObjectStore(ObjectStore):
    """Filesystem-backed store (dev/test). ``key`` segments map to nested directories."""

    def __init__(self, root: str) -> None:
        self._root = Path(root)
        self._root.mkdir(parents=True, exist_ok=True)

    def _path(self, key: str) -> Path:
        # Normalize the key into a path under root; reject traversal out of the root.
        rel = Path(key.lstrip("/"))
        target = (self._root / rel).resolve()
        if not str(target).startswith(str(self._root.resolve())):
            raise ValueError("object key escapes the store root")
        return target

    def put(self, key: str, data: bytes, *, content_type: str | None = None) -> None:
        path = self._path(key)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def get(self, key: str) -> bytes:
        return self._path(key).read_bytes()


class S3ObjectStore(ObjectStore):
    """AWS S3-backed store (production seam; lazy ``boto3`` import — not exercised locally)."""

    def __init__(self, *, bucket: str, region: str = "", endpoint_url: str = "") -> None:
        import boto3  # lazy: boto3 is only needed when S3 mode is selected

        self._bucket = bucket
        kwargs: dict[str, Any] = {}
        if region:
            kwargs["region_name"] = region
        if endpoint_url:
            kwargs["endpoint_url"] = endpoint_url
        self._client = boto3.client("s3", **kwargs)

    def put(self, key: str, data: bytes, *, content_type: str | None = None) -> None:
        extra: dict[str, Any] = {}
        if content_type:
            extra["ContentType"] = content_type
        self._client.put_object(Bucket=self._bucket, Key=key, Body=data, **extra)

    def get(self, key: str) -> bytes:
        resp = self._client.get_object(Bucket=self._bucket, Key=key)
        body = resp["Body"].read()
        return bytes(body)


def build_object_store(settings: Settings) -> ObjectStore:
    if settings.object_store_mode == "s3":
        return S3ObjectStore(
            bucket=settings.s3_bucket,
            region=settings.s3_region,
            endpoint_url=settings.s3_endpoint_url,
        )
    return LocalObjectStore(os.path.expanduser(settings.local_object_store_dir))
