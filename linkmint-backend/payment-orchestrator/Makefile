# payment-orchestrator (work02). The reference Go/chi service Makefile for the LinkMint app layer.

.PHONY: build test test-integration cover lint fmt run tidy

build:
	go build ./...

# Fast unit tests (no Docker required).
test:
	go test ./... -count=1

# Unit + testcontainers integration tests (requires a Docker daemon).
# Ryuk is disabled because every container is explicitly Terminated in t.Cleanup.
test-integration:
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./... -count=1 -p 1 -timeout=500s

# Combined coverage (unit + integration). The DoD coverage gate (>=80%) is measured here.
cover:
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./... -count=1 -p 1 -timeout=500s -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)

fmt:
	gofmt -w .

tidy:
	go mod tidy

run:
	go run ./cmd/payment-orchestrator
