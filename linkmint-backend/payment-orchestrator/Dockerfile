# payment-orchestrator (work02) — Go/chi service image.
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/payment-orchestrator ./cmd/payment-orchestrator

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/payment-orchestrator /usr/local/bin/payment-orchestrator
EXPOSE 8080
ENTRYPOINT ["payment-orchestrator"]
