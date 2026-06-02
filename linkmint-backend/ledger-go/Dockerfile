# ledger-migrate (work16) — one-shot: applies the shared `ledger` schema, then exits.
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/ledger-migrate ./cmd/ledger-migrate

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/ledger-migrate /usr/local/bin/ledger-migrate
ENTRYPOINT ["ledger-migrate"]
