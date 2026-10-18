GO ?= go
MPV ?= mpv
COVERAGE_MIN ?= 90
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -X main.version=$(VERSION)

# Apply the deployment target to ALL CGO packages (including runtime/cgo), not
# just macmedia's #cgo directives. Explicit flags also invalidate cached objects
# compiled for the host SDK's default minimum. Filter old flags on recursive make.
ifeq ($(shell uname -s),Darwin)
export MACOSX_DEPLOYMENT_TARGET := 13.0
export CGO_CFLAGS := $(filter-out -mmacosx-version-min=%,$(shell $(GO) env CGO_CFLAGS)) -mmacosx-version-min=13.0
export CGO_CXXFLAGS := $(filter-out -mmacosx-version-min=%,$(shell $(GO) env CGO_CXXFLAGS)) -mmacosx-version-min=13.0
export CGO_LDFLAGS := $(filter-out -mmacosx-version-min=%,$(shell $(GO) env CGO_LDFLAGS)) -mmacosx-version-min=13.0
endif

.PHONY: native-macos native-media-test native-preflight native-verify help fmt fmt-check mod-check vet test coverage race mpv-test mpris-test check build dist clean

help:
	@printf '%s\n' 'make check    Format, modules, vet, tests, and coverage gate' \
		'make race     Race-detector tests (native platform, requires C compiler)' \
		'make mpv-test Real mpv checks with null audio (requires mpv)' \
		'make mpris-test Private D-Bus protocol checks (requires dbus-daemon)' \
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
	$(GO) test -count=1 -covermode=atomic -coverpkg=$$($(GO) list ./... | paste -sd, -) -coverprofile=coverage/coverage.out ./...
	$(GO) tool cover -func=coverage/coverage.out
	$(GO) tool cover -html=coverage/coverage.out -o coverage/index.html
	@awk -v min='$(COVERAGE_MIN)' 'NR > 1 { statements[$$1] = $$2; if ($$3 > 0) hit[$$1] = 1 } \
		END { for (block in statements) { total += statements[block]; if (hit[block]) covered += statements[block] } \
		if (total == 0) { print "No instrumented statements"; exit 1 } \
		pct = 100 * covered / total; printf "Total coverage: %.2f%%\n", pct; \
		if (pct < min) { printf "Coverage must be >= %s%%\n", min; exit 1 } }' coverage/coverage.out

race:
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

mpv-test:
	PODMUTT_TEST_MPV='$(MPV)' $(GO) test -count=1 -run '^TestRealMPV' -timeout=60s ./internal/player ./internal/app

mpris-test:
	PODMUTT_TEST_DBUS=1 CGO_ENABLED=1 $(GO) test -race -count=1 -run '^TestPrivateBus' -timeout=60s ./internal/mpris

.PHONY: recovery-test
recovery-test:
	$(GO) test -count=1 -run '^TestOfflineWorkflow' -timeout=30s ./internal/app
	$(GO) test -count=1 -run '^Test(AbruptDeathWAL|FailedUpgrade)' -timeout=30s ./internal/store

check: fmt-check mod-check vet coverage

build:
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o bin/podmutt ./cmd/podmutt

# Public MediaPlayer integration requires Apple SDK + CGO + a bundle identity.
# Run this executable from a terminal, not Finder; it is still a TUI.
native-preflight:
	@test "$$(uname -s)" = Darwin || { echo 'Native media checks require macOS 13+ and Apple SDK; cross-builds do not qualify'; exit 1; }
	@test "$$(sw_vers -productVersion | cut -d. -f1)" -ge 13 || { echo 'Go 1.27 requires macOS 13+'; exit 1; }
	xcrun --find clang
	xcrun --show-sdk-path
	$(GO) env CGO_CFLAGS CGO_CXXFLAGS CGO_LDFLAGS
	$(GO) version
	@printf 'Source: %s\n' '$(VERSION)'
	@printf 'Native host: '; uname -m; sw_vers

native-verify: native-preflight
	CGO_ENABLED=1 $(MAKE) check race
	$(MAKE) native-macos native-media-test
	PODMUTT_TEST_MPV='$(MPV)' CGO_ENABLED=1 $(MAKE) mpv-test

native-macos: native-preflight
	@mkdir -p bin/podmutt.app/Contents/MacOS
	cp packaging/macos/Info.plist bin/podmutt.app/Contents/Info.plist
	CGO_ENABLED=1 $(GO) build -trimpath -ldflags='$(LDFLAGS)' -o bin/podmutt.app/Contents/MacOS/podmutt ./cmd/podmutt
	codesign --force --sign - bin/podmutt.app

native-media-test: native-preflight
	@mkdir -p bin/media-test.app/Contents/MacOS
	cp packaging/macos/Info.plist bin/media-test.app/Contents/Info.plist
	/usr/libexec/PlistBuddy -c 'Set :CFBundleIdentifier io.gitlab.susamn.podmutt.media-test' bin/media-test.app/Contents/Info.plist
	CGO_ENABLED=1 $(GO) test -tags=nativecheck -c -o bin/media-test.app/Contents/MacOS/podmutt ./internal/macmedia
	codesign --force --sign - bin/media-test.app
	PODMUTT_TEST_NATIVE_MACOS=1 bin/media-test.app/Contents/MacOS/podmutt -test.run '^TestNative' -test.v -test.timeout=30s

# Cross-built Darwin CLI archives intentionally lack native media keys. Native
# release packaging must use native-macos and the verification matrix in docs.
dist:
	@set -eu; \
	for os in linux darwin; do \
		for arch in amd64 arm64; do \
			name="podmutt-$$os-$$arch"; \
			mkdir -p "dist/$$name"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build ./...; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath \
				-ldflags='$(LDFLAGS)' -o "dist/$$name/podmutt" ./cmd/podmutt; \
			tar -czf "dist/$$name.tar.gz" -C "dist/$$name" podmutt; \
		done; \
	done

clean:
	rm -rf bin dist coverage
