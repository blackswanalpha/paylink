"""Trusted-network routes (ADR-009): the work15-bus intake stand-in + a delivery-status read.

``POST /v1/notifications`` is the chokepoint the future bus subscriber also calls (via
``NotificationEventConsumer.handle``); it honors ``Idempotency-Key`` for HTTP-retry dedupe.
``GET /internal/deliveries/{id}`` is an ops/verification read (recipient masked) — NOT the Phase-2
public delivery-log API.
"""

from __future__ import annotations

import uuid

from fastapi import APIRouter
from fastapi.responses import JSONResponse
from linkmint_idempotency import fingerprint

from app.api.v1.schemas import DeliveryView, NotificationIntakeRequest
from app.deps import ConsumerDep, IdemKey, IdempotencyDep, InternalGateDep, RepoDep
from app.errors import AppError, ErrorCode
from app.redaction import mask_recipient

router = APIRouter()

_INTAKE_ROUTE = "notifications"


@router.post("/v1/notifications", status_code=201)
async def intake(
    body: NotificationIntakeRequest,
    _gate: InternalGateDep,
    consumer: ConsumerDep,
    idem: IdempotencyDep,
    idempotency_key: IdemKey = None,
) -> JSONResponse:
    fp = fingerprint(body.model_dump(mode="json"))
    if idempotency_key:
        cached = await idem.begin(_INTAKE_ROUTE, idempotency_key, fp)
        if cached is not None:
            return JSONResponse(status_code=cached.http_status, content=cached.body)

    try:
        payload: dict[str, object] = {
            "user_id": body.user_id,
            "recipient_addr": body.recipient_addr,
            "locale": body.locale,
            "data": body.data,
            "contact": body.contact.model_dump(exclude_none=True) if body.contact else None,
            "title": body.title,
            "body": body.body,
            "href": body.href,
        }
        ids = await consumer.handle(body.event_kind, payload)
        result = {"delivery_ids": [str(delivery_id) for delivery_id in ids]}
    except Exception:
        if idempotency_key:
            await idem.release(_INTAKE_ROUTE, idempotency_key)
        raise

    if idempotency_key:
        await idem.complete(_INTAKE_ROUTE, idempotency_key, fp, 201, result)
    return JSONResponse(status_code=201, content=result)


@router.get("/internal/deliveries/{delivery_id}", response_model=DeliveryView)
async def get_delivery(
    delivery_id: uuid.UUID,
    _gate: InternalGateDep,
    repo: RepoDep,
) -> DeliveryView:
    row = await repo.get_delivery(delivery_id)
    if row is None:
        raise AppError(
            ErrorCode.DELIVERY_NOT_FOUND,
            "delivery not found",
            details={"delivery_id": str(delivery_id)},
        )
    return DeliveryView(
        delivery_id=str(row.delivery_id),
        channel=row.channel,
        recipient=mask_recipient(row.channel, row.recipient),
        event_kind=row.event_kind,
        status=row.status,
        attempts=row.attempts,
        last_error=row.last_error,
        next_retry_at=row.next_retry_at,
        created_at=row.created_at,
        delivered_at=row.delivered_at,
    )
