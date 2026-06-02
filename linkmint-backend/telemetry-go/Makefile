# telemetry-go (work18). Shared OpenTelemetry helper; mirrors the eventbus-go reference Makefile.
# No integration tests / Docker required — the lib is exercised with in-process unit tests.

.PHONY: build test cover lint fmt tidy

build:
	go build ./...

test:
	go test ./... -count=1

# Coverage gate (>=80%) measured here. Profiles each package with tests individually, then
# concatenates — avoids the cross-package `covdata` merge unavailable on this toolchain.
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
