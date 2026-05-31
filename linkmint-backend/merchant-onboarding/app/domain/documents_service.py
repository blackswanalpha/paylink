"""Document upload. Bytes go to the object store; the DB keeps only the ``s3_key``.

There is no spec event for a document upload (per backendfeatures.md §2.10), so this only logs.
Enforces the ``MERCHANT_MAX_DOCUMENT_BYTES`` cap → ``PAYLOAD_TOO_LARGE`` (413).
"""

from __future__ import annotations

import uuid
from collections.abc import Awaitable, Callable

from app.config import Settings
from app.db.models import DocumentRow
from app.db.repositories import MerchantRepository
from app.domain import rbac
from app.domain.models import DocumentKind
from app.errors import AppError, ErrorCode
from app.logging import get_logger
from app.security.jwt import AccessClaims
from app.storage.object_store import ObjectStore

log = get_logger("merchant.documents")

_Commit = Callable[[], Awaitable[None]]


class DocumentsService:
    def __init__(
        self,
        repo: MerchantRepository,
        commit: _Commit,
        object_store: ObjectStore,
        settings: Settings,
    ) -> None:
        self._repo = repo
        self._commit = commit
        self._object_store = object_store
        self._settings = settings

    async def upload_document(
        self,
        *,
        principal: AccessClaims,
        merchant_id: uuid.UUID,
        kind: str,
        filename: str,
        content: bytes,
        content_type: str | None = None,
    ) -> DocumentRow:
        merchant = await self._repo.get_merchant(merchant_id)
        if merchant is None or rbac.org_role(principal, str(merchant.org_id)) is None:
            raise AppError(ErrorCode.MERCHANT_NOT_FOUND, "merchant not found")
        if kind not in set(DocumentKind):
            raise AppError(ErrorCode.INVALID_PAYLOAD, f"invalid document kind '{kind}'")
        if len(content) > self._settings.max_document_bytes:
            raise AppError(
                ErrorCode.PAYLOAD_TOO_LARGE,
                "document exceeds the maximum allowed size",
                details={"max_bytes": self._settings.max_document_bytes},
            )

        document_id = uuid.uuid4()
        s3_key = f"{self._settings.s3_prefix}/{merchant_id}/{document_id}"
        self._object_store.put(s3_key, content, content_type=content_type)
        document = DocumentRow(
            document_id=document_id,
            merchant_id=merchant_id,
            kind=kind,
            s3_key=s3_key,
            review=None,
        )
        await self._repo.insert_document(document)
        await self._commit()
        # No spec event for documents — log only (filename is metadata, not document content).
        log.info(
            "document_uploaded",
            merchant_id=str(merchant_id),
            document_id=str(document_id),
            kind=kind,
            filename=filename,
            bytes=len(content),
        )
        return document
