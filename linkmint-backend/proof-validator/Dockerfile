# proof-validator (work03) — Go/chi service image.
#
# Build context MUST be the monorepo root: this module replaces github.com/paylink/paylink-chain
# with ../../paylink-chain (to reuse pkg/lvm byte-exact), so BOTH module trees must be present in
# the build context. docker-compose sets `build.context: .` with this dockerfile path; building by
# hand: `docker build -f linkmint-backend/proof-validator/Dockerfile .` from the repo root.
FROM golang:1.25 AS build
WORKDIR /src
# Copy both modules, preserving the relative layout so `replace ../../paylink-chain` resolves.
COPY paylink-chain/ ./paylink-chain/
COPY linkmint-backend/proof-validator/ ./linkmint-backend/proof-validator/
WORKDIR /src/linkmint-backend/proof-validator
RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/proof-validator ./cmd/proof-validator

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/proof-validator /usr/local/bin/proof-validator
EXPOSE 8081
ENTRYPOINT ["proof-validator"]
