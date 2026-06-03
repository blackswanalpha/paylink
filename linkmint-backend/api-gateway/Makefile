SHELL := /bin/bash
IMAGE := kong:3.7
COMPOSE_TEST := docker compose -f test/docker-compose.test.yml
ROOT_COMPOSE := docker compose -f ../../docker-compose.yml

# Render defaults — override on the CLI or via the environment / .env.
export GATEWAY_PAYLINK_SERVICE_URL ?= http://paylink-service:8000
export GATEWAY_PAYMENT_ORCHESTRATOR_URL ?= http://payment-orchestrator:8080
export GATEWAY_NOTIFICATION_SERVICE_URL ?= http://notification-service:8095
# work08 — additive pass-through upstreams (identity/merchant/compliance/admin/audit).
export GATEWAY_IDENTITY_SERVICE_URL ?= http://identity-service:8090
export GATEWAY_MERCHANT_SERVICE_URL ?= http://merchant-onboarding:8091
export GATEWAY_COMPLIANCE_SERVICE_URL ?= http://compliance-risk:8093
export GATEWAY_ADMIN_SERVICE_URL ?= http://admin-backoffice:8092
export GATEWAY_AUDIT_SERVICE_URL ?= http://audit-log-service:8094
# work21 — additive pass-through upstream (fee-pricing).
export GATEWAY_FEE_PRICING_SERVICE_URL ?= http://fee-pricing-service:8097
export GATEWAY_JWT_ISSUER ?= linkmint-dev
export GATEWAY_JWT_DEV_SECRET ?= devsecret-change-me
export GATEWAY_JWT_CREATOR_ADDR_CLAIM ?= creator_addr
export GATEWAY_PARTNER_API_KEY ?= devpartnerkey-change-me
export GATEWAY_PARTNER_CREATOR_ADDR ?= 0x00000000000000000000000000000000000000bb
export GATEWAY_RATE_LIMIT_PER_MINUTE ?= 100
export GATEWAY_REDIS_HOST ?= redis
export GATEWAY_REDIS_PORT ?= 6379
# work09 RS256 seam (additive) — empty until provisioned; only used if the identity-rs256 consumer
# in kong.yml.tmpl is uncommented. Kept in the allowlist so activation is just an uncomment + set.
export GATEWAY_JWT_RS256_ISSUER ?=
export GATEWAY_JWT_RS256_PUBLIC_KEY ?=

VARS := $$GATEWAY_PAYLINK_SERVICE_URL $$GATEWAY_PAYMENT_ORCHESTRATOR_URL \
  $$GATEWAY_NOTIFICATION_SERVICE_URL \
  $$GATEWAY_IDENTITY_SERVICE_URL $$GATEWAY_MERCHANT_SERVICE_URL $$GATEWAY_COMPLIANCE_SERVICE_URL \
  $$GATEWAY_ADMIN_SERVICE_URL $$GATEWAY_AUDIT_SERVICE_URL $$GATEWAY_FEE_PRICING_SERVICE_URL \
  $$GATEWAY_JWT_ISSUER \
  $$GATEWAY_JWT_DEV_SECRET $$GATEWAY_JWT_CREATOR_ADDR_CLAIM $$GATEWAY_PARTNER_API_KEY \
  $$GATEWAY_PARTNER_CREATOR_ADDR $$GATEWAY_RATE_LIMIT_PER_MINUTE $$GATEWAY_REDIS_HOST \
  $$GATEWAY_REDIS_PORT $$GATEWAY_JWT_RS256_ISSUER $$GATEWAY_JWT_RS256_PUBLIC_KEY

.PHONY: render validate build test run down clean help

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n",$$1,$$2}'

render: ## Render kong.yml.tmpl -> kong/kong.yml from the current env
	envsubst '$(VARS)' < kong/kong.yml.tmpl > kong/kong.yml
	@echo "rendered kong/kong.yml"

validate: render ## Validate the rendered declarative config with Kong
	docker run --rm -e KONG_DATABASE=off -v "$(CURDIR)/kong:/cfg:ro" $(IMAGE) kong config parse /cfg/kong.yml

build: ## Build the gateway image
	$(COMPOSE_TEST) build

test: ## Boot the isolated test stack (gateway + redis + echo) and run the pytest matrix
	$(COMPOSE_TEST) up -d --build --wait
	python3 -m venv test/.venv && test/.venv/bin/pip -q install -r test/requirements.txt
	- test/.venv/bin/python -m pytest test -v
	$(COMPOSE_TEST) down -v

run: ## Bring the gateway up in front of the real services (repo-root compose)
	$(ROOT_COMPOSE) up -d --build api-gateway

down: ## Tear down the isolated test stack
	$(COMPOSE_TEST) down -v

clean: ## Remove the rendered config + test venv
	rm -rf kong/kong.yml test/.venv
