"""`GET /v1/fx/rates` — cached FX rates. JWT (any authenticated caller).

With ``?base=&quote=`` returns that one pair; otherwise returns every configured pair. Each lookup
goes through the FX service (cache → provider → fallback), so the first read warms the 60s cache.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Query

from app.api.v1 import schemas
from app.deps import PrincipalDep, ServicesDep, SettingsDep
from app.fx.static import parse_rate_table

router = APIRouter(prefix="/v1/fx", tags=["fx"])


@router.get("/rates", response_model=schemas.FxRatesResponse)
async def get_rates(
    services: ServicesDep,
    settings: SettingsDep,
    principal: PrincipalDep,
    base: Annotated[str | None, Query()] = None,
    quote: Annotated[str | None, Query()] = None,
) -> schemas.FxRatesResponse:
    if base and quote:
        pairs = [(base.upper(), quote.upper())]
    else:
        # All configured static pairs (deterministic order); identity pairs resolve to 1.
        pairs = sorted(parse_rate_table(settings.fx_static_rates).keys())
    rates = []
    for b, q in pairs:
        r = await services.fx.rate_for(b, q)
        rates.append(
            schemas.FxRateResponse(
                base=r.base,
                quote=r.quote,
                rate=str(r.rate),
                source=r.source,
                fetched_at=r.fetched_at,
            )
        )
    return schemas.FxRatesResponse(rates=rates)
