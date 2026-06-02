# payment-orchestrator (work02) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/eventbus-go with
# ../eventbus-go (work15) and github.com/paylink/idempotency-go with ../idempotency-go (work17), so
# those module trees must be present in the build context. docker-compose sets `build.context: .` with
# this dockerfile path; by hand: `docker build -f linkmint-backend/payment-orchestrator/Dockerfile .`
FROM golang:1.25 AS build
WORKDIR /src
# Copy the sibling modules the binary depends on, preserving the relative layout so the replaces resolve.
COPY linkmint-backend/eventbus-go/ ./linkmint-backend/eventbus-go/
COPY linkmint-backend/idempotency-go/ ./linkmint-backend/idempotency-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY linkmint-backend/payment-orchestrator/ ./linkmint-backend/payment-orchestrator/
WORKDIR /src/linkmint-backend/payment-orchestrator
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/payment-orchestrator ./cmd/payment-orchestrator

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/payment-orchestrator /usr/local/bin/payment-orchestrator
EXPOSE 8080
ENTRYPOINT ["payment-orchestrator"]
