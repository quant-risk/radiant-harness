# Radiant Harness — Makefile
.PHONY: build test lint clean install release smoke test-agents

# CGO_ENABLED=0 is required on macOS arm64 + Go 1.22.x to avoid the
# "dyld: missing LC_UUID load command" abort trap. The Dockerfile already
# uses CGO_ENABLED=0; this keeps `make` consistent with `docker build`.
CGO_ENABLED ?= 0

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

# Default: build the radiant binary.
build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o bin/radiant ./cmd/radiant/

# Cross-platform release. All six OS/arch targets the harness advertises:
#   linux/amd64     — x86 servers, CI, Docker on x86 hosts
#   linux/arm64     — AWS Graviton, Raspberry Pi 4/5, Linux ARM servers
#   darwin/amd64    — Intel Mac (rare but still in the wild)
#   darwin/arm64    — Apple Silicon (M1/M2/M3/M4)
#   windows/amd64   — x86 Windows, WSL2 default, most cloud VMs
#   windows/arm64   — Surface Pro X, ARM-native Windows
release:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/radiant-linux-amd64     ./cmd/radiant/
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o dist/radiant-linux-arm64     ./cmd/radiant/
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/radiant-darwin-amd64    ./cmd/radiant/
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/radiant-darwin-arm64    ./cmd/radiant/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/radiant-windows-amd64.exe ./cmd/radiant/
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/radiant-windows-arm64.exe ./cmd/radiant/
	@echo "✓ Release binaries in dist/"

# Tests (full project)
test:
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -count=1

test-short:
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -short

lint:
	CGO_ENABLED=$(CGO_ENABLED) go vet ./...
	@echo "✓ vet passed"

clean:
	rm -rf bin/ dist/

install: build
	install -m 0755 bin/radiant $(DESTDIR)/usr/local/bin/radiant

# Smoke test: builds, runs setup-mcp --help, host-info, verifies zero
# HTTP-LLM symbols, checks size.
smoke: build
	./scripts/smoke-test.sh

# Cross-agent install matrix (Sprint 5). Builds first, then for each of
# the 12 supported host agents: simulates the host's env, runs setup-mcp
# in a sandbox HOME + sandbox proj/, runs doctor --mcp and mcp self-test
# against the resulting config, and emits a Markdown pass/fail report.
# Manual invocation; no cron, no polling, no daemon.
test-agents: build
	./scripts/test-agents.sh