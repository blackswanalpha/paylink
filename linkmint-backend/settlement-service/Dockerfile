# settlement-service (work23) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/eventbus-go with
# ../eventbus-go (work15), github.com/paylink/idempotency-go with ../idempotency-go (work17),
# github.com/paylink/ledger-go with ../ledger-go (work16), and github.com/paylink/telemetry-go with
# ../telemetry-go (work18), so those module trees must be present in the build context. docker-compose
# sets `build.context: .` with this dockerfile path; by hand:
# `docker build -f linkmint-backend/settlement-service/Dockerfile .`
FROM golang:1.25 AS build
WORKDIR /src
# Copy the sibling modules the binary depends on, preserving the relative layout so the replaces resolve.
COPY linkmint-backend/eventbus-go/ ./linkmint-backend/eventbus-go/
COPY linkmint-backend/idempotency-go/ ./linkmint-backend/idempotency-go/
COPY linkmint-backend/ledger-go/ ./linkmint-backend/ledger-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY linkmint-backend/settlement-service/ ./linkmint-backend/settlement-service/
WORKDIR /src/linkmint-backend/settlement-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/settlement-service ./cmd/settlement-service

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/settlement-service /usr/local/bin/settlement-service
EXPOSE 8101
ENTRYPOINT ["settlement-service"]
