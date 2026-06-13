# wallet-service (work24) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/paylink-chain
# with ../../paylink-chain (to reuse pkg/lvm byte-exact for the unsigned-tx intents),
# github.com/paylink/eventbus-go with ../eventbus-go (work15), github.com/paylink/idempotency-go
# with ../idempotency-go (work17), and github.com/paylink/telemetry-go with ../telemetry-go
# (work18), so those module trees must be present in the build context. There is NO ledger-go
# dependency — wallet-service is read-side and records no money flows.
# docker-compose sets `build.context: .` with this dockerfile path; building by hand:
# `docker build -f linkmint-backend/wallet-service/Dockerfile .` from the repo root.
FROM golang:1.25 AS build
WORKDIR /src
# Copy the modules, preserving the relative layout so the replaces resolve.
COPY paylink-chain/ ./paylink-chain/
COPY linkmint-backend/eventbus-go/ ./linkmint-backend/eventbus-go/
COPY linkmint-backend/idempotency-go/ ./linkmint-backend/idempotency-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY linkmint-backend/wallet-service/ ./linkmint-backend/wallet-service/
WORKDIR /src/linkmint-backend/wallet-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/wallet-service ./cmd/wallet-service

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/wallet-service /usr/local/bin/wallet-service
EXPOSE 8102
ENTRYPOINT ["wallet-service"]
