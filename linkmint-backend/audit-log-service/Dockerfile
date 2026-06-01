# audit-log-service (work13) — Go/chi service image.
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/audit-log-service ./cmd/audit-log-service

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/audit-log-service /usr/local/bin/audit-log-service
EXPOSE 8094
ENTRYPOINT ["audit-log-service"]
