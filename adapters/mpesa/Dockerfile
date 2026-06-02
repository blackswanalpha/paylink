# mpesa-adapter CORE (work04) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/paylink-chain
# with ../../paylink-chain (to reuse pkg/lvm byte-exact) and github.com/paylink/idempotency-go with
# ../../linkmint-backend/idempotency-go (work17), so those module trees must be present.
# docker-compose sets `build.context: .` with this dockerfile path; by hand:
#   docker build -f adapters/mpesa/Dockerfile .
# (The Node Daraja rail SDK is a separate image — see adapters/mpesa/daraja-service/Dockerfile.)
FROM golang:1.25 AS build
WORKDIR /src
# Copy the modules, preserving the relative layout so the replaces resolve.
COPY paylink-chain/ ./paylink-chain/
COPY linkmint-backend/idempotency-go/ ./linkmint-backend/idempotency-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY adapters/mpesa/ ./adapters/mpesa/
WORKDIR /src/adapters/mpesa
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/mpesa-adapter ./cmd/mpesa-adapter

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/mpesa-adapter /usr/local/bin/mpesa-adapter
EXPOSE 8082
ENTRYPOINT ["mpesa-adapter"]
