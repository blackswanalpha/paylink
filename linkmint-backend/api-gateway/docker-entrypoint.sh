#!/usr/bin/env bash
# Render the DB-less declarative config from the environment (12-factor), then start Kong.
# Only our known GATEWAY_* placeholders are substituted, so the embedded ($-free) Lua is untouched.
set -euo pipefail

# Defaults (compose/.env override these). Secrets default to empty — set them in the environment.
: "${GATEWAY_PAYLINK_SERVICE_URL:=http://paylink-service:8000}"
: "${GATEWAY_PAYMENT_ORCHESTRATOR_URL:=http://payment-orchestrator:8080}"
: "${GATEWAY_JWT_ISSUER:=linkmint-dev}"
: "${GATEWAY_JWT_DEV_SECRET:=}"
: "${GATEWAY_JWT_CREATOR_ADDR_CLAIM:=creator_addr}"
: "${GATEWAY_PARTNER_API_KEY:=}"
: "${GATEWAY_PARTNER_CREATOR_ADDR:=0x00000000000000000000000000000000000000bb}"
: "${GATEWAY_RATE_LIMIT_PER_MINUTE:=100}"
: "${GATEWAY_REDIS_HOST:=redis}"
: "${GATEWAY_REDIS_PORT:=6379}"

export GATEWAY_PAYLINK_SERVICE_URL GATEWAY_PAYMENT_ORCHESTRATOR_URL GATEWAY_JWT_ISSUER \
  GATEWAY_JWT_DEV_SECRET GATEWAY_JWT_CREATOR_ADDR_CLAIM GATEWAY_PARTNER_API_KEY \
  GATEWAY_PARTNER_CREATOR_ADDR GATEWAY_RATE_LIMIT_PER_MINUTE GATEWAY_REDIS_HOST GATEWAY_REDIS_PORT

TMPL="${GATEWAY_CONFIG_TEMPLATE:-/kong/kong.yml.tmpl}"
OUT="${KONG_DECLARATIVE_CONFIG:-/kong/kong.yml}"

envsubst '
  ${GATEWAY_PAYLINK_SERVICE_URL} ${GATEWAY_PAYMENT_ORCHESTRATOR_URL} ${GATEWAY_JWT_ISSUER}
  ${GATEWAY_JWT_DEV_SECRET} ${GATEWAY_JWT_CREATOR_ADDR_CLAIM} ${GATEWAY_PARTNER_API_KEY}
  ${GATEWAY_PARTNER_CREATOR_ADDR} ${GATEWAY_RATE_LIMIT_PER_MINUTE} ${GATEWAY_REDIS_HOST}
  ${GATEWAY_REDIS_PORT}
' < "$TMPL" > "$OUT"

echo "[entrypoint] rendered $OUT (paylink=$GATEWAY_PAYLINK_SERVICE_URL payments=$GATEWAY_PAYMENT_ORCHESTRATOR_URL rate=${GATEWAY_RATE_LIMIT_PER_MINUTE}/min)"

# Hand off to the stock Kong entrypoint (CMD is `kong docker-start`).
exec /docker-entrypoint.sh "$@"
