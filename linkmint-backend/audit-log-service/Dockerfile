# audit-log-service (work13) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/idempotency-go
# with ../idempotency-go (work17), so that module tree must be present in the build context.
# docker-compose sets `build.context: .` with this dockerfile path; by hand:
#   docker build -f linkmint-backend/audit-log-service/Dockerfile .
FROM golang:1.25 AS build
WORKDIR /src
# Copy the sibling module the binary depends on, preserving the relative layout so the replace resolves.
COPY linkmint-backend/idempotency-go/ ./linkmint-backend/idempotency-go/
COPY linkmint-backend/telemetry-go/ ./linkmint-backend/telemetry-go/
COPY linkmint-backend/audit-log-service/ ./linkmint-backend/audit-log-service/
WORKDIR /src/linkmint-backend/audit-log-service
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/audit-log-service ./cmd/audit-log-service

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/audit-log-service /usr/local/bin/audit-log-service
EXPOSE 8094
ENTRYPOINT ["audit-log-service"]
