# Single-validator lVM devnet image (used by docker-compose --profile e2e for read-through e2e).
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/paylinkd ./cmd/paylinkd

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/paylinkd /usr/local/bin/paylinkd
RUN mkdir -p /data
EXPOSE 8545 9000
ENTRYPOINT ["paylinkd"]
CMD ["--datadir", "/data", "--rpc", ":8545", "--block-interval", "1000"]
