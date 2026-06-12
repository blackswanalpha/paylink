# escrow-manager (work20). Go/chi service; mirrors the work03 proof-validator Makefile.

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
# Profiles each package that has tests individually, then concatenates the profiles. This avoids
# the cross-package coverage merge (which shells out to the `covdata` tool) so it works on
# toolchains whose `covdata` is unavailable; it also naturally excludes the no-test main package.
cover:
	@echo "mode: atomic" > coverage.out
	@for pkg in $$(go list -tags=integration -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...); do \
		TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration -covermode=atomic -coverprofile=cov.pkg.out -count=1 -timeout=500s "$$pkg" || exit 1; \
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
	go run ./cmd/escrow-manager
