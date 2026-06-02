# chain-event-mirror (work15) — Go service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/eventbus-go with
# ../eventbus-go (to reuse the shared Kafka client lib), so BOTH module trees must be present in the
# build context. docker-compose sets `build.context: .` with this dockerfile path; building by hand:
# `docker build -f linkmint-backend/chain-event-mirror/Dockerfile .` from the repo root.
FROM golang:1.25 AS build
WORKDIR /src
# Copy both modules, preserving the relative layout so `replace ../eventbus-go` resolves.
COPY linkmint-backend/eventbus-go/ ./linkmint-backend/eventbus-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY linkmint-backend/chain-event-mirror/ ./linkmint-backend/chain-event-mirror/
WORKDIR /src/linkmint-backend/chain-event-mirror
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/chain-event-mirror ./cmd/chain-event-mirror

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/chain-event-mirror /usr/local/bin/chain-event-mirror
EXPOSE 8096
ENTRYPOINT ["chain-event-mirror"]
