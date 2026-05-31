# mpesa-adapter CORE (work04). Go/chi service; mirrors the work03 proof-validator Makefile.
# The Daraja rail SDK is a separate Node service (daraja-service/) with its own npm scripts.

.PHONY: build test cover lint fmt run tidy e2e

build:
	go build ./...

# Unit tests (no Docker, no network — Daraja and the validator are faked).
test:
	go test ./... -count=1

# Per-package coverage, concatenated. Profiling each package individually avoids the cross-package
# coverage merge (which shells out to `covdata`, unavailable on this env's toolchains) and naturally
# excludes the no-test main package.
cover:
	@echo "mode: atomic" > coverage.out
	@for pkg in $$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...); do \
		go test -covermode=atomic -coverprofile=cov.pkg.out -count=1 "$$pkg" || exit 1; \
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
	go run ./cmd/mpesa-adapter

# End-to-end against `docker compose --profile e2e`. Requires the full stack up (chain, validator,
# redis, mpesa-adapter, mpesa-daraja). See test/e2e_settlement_test.go.
e2e:
	go test -tags=e2e ./test/... -count=1 -v
