#!/usr/bin/env bash
# Render the DB-less declarative config from the environment (12-factor), then start Kong.
# Only our known GATEWAY_* placeholders are substituted, so the embedded ($-free) Lua is untouched.
set -euo pipefail

# Defaults (compose/.env override these). Secrets default to empty — set them in the environment.
: "${GATEWAY_PAYLINK_SERVICE_URL:=http://paylink-service:8000}"
: "${GATEWAY_PAYMENT_ORCHESTRATOR_URL:=http://payment-orchestrator:8080}"
: "${GATEWAY_NOTIFICATION_SERVICE_URL:=http://notification-service:8095}"
# work08 — additive pass-through upstreams (identity/merchant/compliance/admin/audit).
: "${GATEWAY_IDENTITY_SERVICE_URL:=http://identity-service:8090}"
: "${GATEWAY_MERCHANT_SERVICE_URL:=http://merchant-onboarding:8091}"
: "${GATEWAY_COMPLIANCE_SERVICE_URL:=http://compliance-risk:8093}"
: "${GATEWAY_ADMIN_SERVICE_URL:=http://admin-backoffice:8092}"
: "${GATEWAY_AUDIT_SERVICE_URL:=http://audit-log-service:8094}"
# work21 — additive pass-through upstream (fee-pricing).
: "${GATEWAY_FEE_PRICING_SERVICE_URL:=http://fee-pricing-service:8097}"
# work22 — additive pass-through upstream (refund-dispute).
: "${GATEWAY_REFUND_DISPUTE_SERVICE_URL:=http://refund-dispute-service:8100}"
# work20 — escrow-manager (authenticated like paylinks/payments).
: "${GATEWAY_ESCROW_SERVICE_URL:=http://escrow-manager:8098}"
# work23 — settlement-service (authenticated like paylinks/payments; X-Creator-Addr injected).
: "${GATEWAY_SETTLEMENT_SERVICE_URL:=http://settlement-service:8101}"
: "${GATEWAY_JWT_ISSUER:=linkmint-dev}"
: "${GATEWAY_JWT_DEV_SECRET:=}"
: "${GATEWAY_JWT_CREATOR_ADDR_CLAIM:=creator_addr}"
: "${GATEWAY_PARTNER_API_KEY:=}"
: "${GATEWAY_PARTNER_CREATOR_ADDR:=0x00000000000000000000000000000000000000bb}"
: "${GATEWAY_RATE_LIMIT_PER_MINUTE:=100}"
: "${GATEWAY_REDIS_HOST:=redis}"
: "${GATEWAY_REDIS_PORT:=6379}"
# work09 RS256 seam (additive) — empty until provisioned; only consumed if the identity-rs256
# consumer in kong.yml.tmpl is uncommented.
: "${GATEWAY_JWT_RS256_ISSUER:=}"
: "${GATEWAY_JWT_RS256_PUBLIC_KEY:=}"

export GATEWAY_PAYLINK_SERVICE_URL GATEWAY_PAYMENT_ORCHESTRATOR_URL GATEWAY_NOTIFICATION_SERVICE_URL \
  GATEWAY_IDENTITY_SERVICE_URL GATEWAY_MERCHANT_SERVICE_URL GATEWAY_COMPLIANCE_SERVICE_URL \
  GATEWAY_ADMIN_SERVICE_URL GATEWAY_AUDIT_SERVICE_URL GATEWAY_FEE_PRICING_SERVICE_URL \
  GATEWAY_REFUND_DISPUTE_SERVICE_URL \
  GATEWAY_ESCROW_SERVICE_URL \
  GATEWAY_SETTLEMENT_SERVICE_URL \
  GATEWAY_JWT_ISSUER GATEWAY_JWT_DEV_SECRET GATEWAY_JWT_CREATOR_ADDR_CLAIM GATEWAY_PARTNER_API_KEY \
  GATEWAY_PARTNER_CREATOR_ADDR GATEWAY_RATE_LIMIT_PER_MINUTE GATEWAY_REDIS_HOST GATEWAY_REDIS_PORT \
  GATEWAY_JWT_RS256_ISSUER GATEWAY_JWT_RS256_PUBLIC_KEY

TMPL="${GATEWAY_CONFIG_TEMPLATE:-/kong/kong.yml.tmpl}"
OUT="${KONG_DECLARATIVE_CONFIG:-/kong/kong.yml}"

envsubst '
  ${GATEWAY_PAYLINK_SERVICE_URL} ${GATEWAY_PAYMENT_ORCHESTRATOR_URL} ${GATEWAY_NOTIFICATION_SERVICE_URL}
  ${GATEWAY_IDENTITY_SERVICE_URL} ${GATEWAY_MERCHANT_SERVICE_URL} ${GATEWAY_COMPLIANCE_SERVICE_URL}
  ${GATEWAY_ADMIN_SERVICE_URL} ${GATEWAY_AUDIT_SERVICE_URL} ${GATEWAY_FEE_PRICING_SERVICE_URL}
  ${GATEWAY_REFUND_DISPUTE_SERVICE_URL}
  ${GATEWAY_ESCROW_SERVICE_URL}
  ${GATEWAY_SETTLEMENT_SERVICE_URL}
  ${GATEWAY_JWT_ISSUER} ${GATEWAY_JWT_DEV_SECRET} ${GATEWAY_JWT_CREATOR_ADDR_CLAIM}
  ${GATEWAY_PARTNER_API_KEY} ${GATEWAY_PARTNER_CREATOR_ADDR} ${GATEWAY_RATE_LIMIT_PER_MINUTE}
  ${GATEWAY_REDIS_HOST} ${GATEWAY_REDIS_PORT} ${GATEWAY_JWT_RS256_ISSUER} ${GATEWAY_JWT_RS256_PUBLIC_KEY}
' < "$TMPL" > "$OUT"

echo "[entrypoint] rendered $OUT (paylink=$GATEWAY_PAYLINK_SERVICE_URL payments=$GATEWAY_PAYMENT_ORCHESTRATOR_URL notify=$GATEWAY_NOTIFICATION_SERVICE_URL rate=${GATEWAY_RATE_LIMIT_PER_MINUTE}/min)"

# Hand off to the stock Kong entrypoint (CMD is `kong docker-start`).
exec /docker-entrypoint.sh "$@"
