GO ?= go
COVERAGE_MIN ?= 90
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -X main.version=$(VERSION)

.PHONY: help fmt fmt-check mod-check vet test coverage race check build dist clean

help:
	@printf '%s\n' 'make check    Format, modules, vet, tests, and coverage gate' \
		'make race     Race-detector tests (native platform, requires C compiler)' \
		'make build    Build bin/podmutt' \
		'make dist     Cross-build Linux/macOS amd64/arm64 archives' \
		'make fmt      Format Go source' \
		'make clean    Remove generated artifacts'

fmt:
	@gofmt -w $$($(GO) list -f '{{.Dir}}' ./...)

fmt-check:
	@files=$$(gofmt -l $$($(GO) list -f '{{.Dir}}' ./...)); \
		if [ -n "$$files" ]; then printf 'Run make fmt:\n%s\n' "$$files"; exit 1; fi

mod-check:
	$(GO) mod tidy -diff

vet:
	$(GO) vet ./...

test:
	$(GO) test -count=1 ./...

coverage:
	@mkdir -p coverage
	$(GO) test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage/coverage.out ./...
	$(GO) tool cover -func=coverage/coverage.out
	$(GO) tool cover -html=coverage/coverage.out -o coverage/index.html
	@awk -v min='$(COVERAGE_MIN)' 'NR > 1 { total += $$2; if ($$3 > 0) covered += $$2 } \
		END { if (total == 0) { print "No instrumented statements"; exit 1 } \
		pct = 100 * covered / total; printf "Total coverage: %.2f%%\n", pct; \
		if (pct < min) { printf "Coverage must be >= %s%%\n", min; exit 1 } }' coverage/coverage.out

race:
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

check: fmt-check mod-check vet coverage

build:
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o bin/podmutt ./cmd/podmutt

dist:
	@set -eu; \
	for os in linux darwin; do \
		for arch in amd64 arm64; do \
			name="podmutt-$$os-$$arch"; \
			mkdir -p "dist/$$name"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath \
				-ldflags='$(LDFLAGS)' -o "dist/$$name/podmutt" ./cmd/podmutt; \
			tar -czf "dist/$$name.tar.gz" -C "dist/$$name" podmutt; \
		done; \
	done

clean:
	rm -rf bin dist coverage
