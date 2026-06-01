"""`/v1/merchants/{id}/documents` — multipart upload (cert of incorporation, tax id, …)."""

from __future__ import annotations

import hashlib
from typing import Annotated, Any

from fastapi import APIRouter, File, Form, UploadFile
from fastapi.responses import JSONResponse

from app.api.v1 import schemas
from app.api.v1._helpers import idempotent, parse_uuid
from app.deps import IdemKey, IdempotencyDep, PrincipalDep, ServicesDep

router = APIRouter(prefix="/v1/merchants", tags=["documents"])


@router.post("/{merchant_id}/documents", status_code=201)
async def upload_document(
    merchant_id: str,
    services: ServicesDep,
    principal: PrincipalDep,
    idem: IdempotencyDep,
    file: Annotated[UploadFile, File()],
    kind: Annotated[str, Form()],
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    mid = parse_uuid(merchant_id, field="merchant_id")
    content = await file.read()
    filename = file.filename or "document"
    # Fingerprint over content-addressed metadata (sha256 of the bytes) so a re-uploaded identical
    # file replays, while a different file/kind is a fresh upload (or a 409 under the same key).
    fp_payload = {
        "merchant_id": merchant_id,
        "kind": kind,
        "filename": filename,
        "sha256": hashlib.sha256(content).hexdigest(),
    }

    async def work() -> dict[str, Any]:
        document = await services.documents.upload_document(
            principal=principal,
            merchant_id=mid,
            kind=kind,
            filename=filename,
            content=content,
            content_type=file.content_type,
        )
        return schemas.DocumentResponse(
            document_id=str(document.document_id), status="UPLOADED"
        ).model_dump(mode="json")

    return await idempotent(idem, "upload_document", idempotency_key, fp_payload, 201, work)
