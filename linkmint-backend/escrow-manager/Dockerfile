# escrow-manager (work20) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/eventbus-go with
# ../eventbus-go (work15), github.com/paylink/idempotency-go with ../idempotency-go (work17), and
# github.com/paylink/telemetry-go with ../telemetry-go (work18), so those module trees must be
# present in the build context. docker-compose sets `build.context: .` with this dockerfile path;
# by hand: `docker build -f linkmint-backend/escrow-manager/Dockerfile .`
FROM golang:1.25 AS build
WORKDIR /src
# Copy the sibling modules the binary depends on, preserving the relative layout so the replaces resolve.
COPY linkmint-backend/eventbus-go/ ./linkmint-backend/eventbus-go/
COPY linkmint-backend/idempotency-go/ ./linkmint-backend/idempotency-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY linkmint-backend/escrow-manager/ ./linkmint-backend/escrow-manager/
WORKDIR /src/linkmint-backend/escrow-manager
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/escrow-manager ./cmd/escrow-manager

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/escrow-manager /usr/local/bin/escrow-manager
EXPOSE 8098
ENTRYPOINT ["escrow-manager"]
