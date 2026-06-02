# chain-event-mirror (work15). Go service; mirrors the work02/03 reference Makefile.

.PHONY: build test test-integration cover lint fmt tidy run

build:
	go build ./...

# Fast unit tests (no Docker required).
test:
	go test ./... -count=1

# Unit + testcontainers integration tests (requires a Docker daemon; boots Redpanda).
test-integration:
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./... -count=1 -p 1 -timeout=600s

# Combined coverage (unit + integration). Profiles each package with tests individually, then
# concatenates — avoids the cross-package `covdata` merge unavailable on this toolchain.
cover:
	@echo "mode: atomic" > coverage.out
	@for pkg in $$(go list -tags=integration -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...); do \
		TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration -covermode=atomic -coverprofile=cov.pkg.out -count=1 -timeout=600s "$$pkg" || exit 1; \
		if [ -f cov.pkg.out ]; then tail -n +2 cov.pkg.out >> coverage.out && rm -f cov.pkg.out; fi; \
	done
	@go tool cover -func=coverage.out | tail -1

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)

fmt:
	gofmt -w .

tidy:
	go mod tidy

run:
	go run ./cmd/chain-event-mirror
