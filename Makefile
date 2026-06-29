# Radiant Harness — Makefile
.PHONY: build test lint clean install release light light-all light-smoke

# CGO_ENABLED=0 is required on macOS arm64 + Go 1.22.x to avoid the
# "dyld: missing LC_UUID load command" abort trap. The Dockerfile already
# uses CGO_ENABLED=0; this keeps `make` consistent with `docker build`.
CGO_ENABLED ?= 0

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o bin/radiant ./cmd/radiant/

# Light build: no API key infrastructure whatsoever (Sprint 78+).
# Binary is physically incapable of HTTP LLM — only MCP sampling.
light:
	CGO_ENABLED=$(CGO_ENABLED) go build -tags light_only $(LDFLAGS) -o bin/radiant-light ./cmd/radiant/

# Cross-platform Light release (same platforms as Full).
light-all:
	GOOS=linux   GOARCH=amd64 go build -tags light_only $(LDFLAGS) -o dist/radiant-light-linux-amd64     ./cmd/radiant/
	GOOS=linux   GOARCH=arm64 go build -tags light_only $(LDFLAGS) -o dist/radiant-light-linux-arm64     ./cmd/radiant/
	GOOS=darwin  GOARCH=amd64 go build -tags light_only $(LDFLAGS) -o dist/radiant-light-darwin-amd64    ./cmd/radiant/
	GOOS=darwin  GOARCH=arm64 go build -tags light_only $(LDFLAGS) -o dist/radiant-light-darwin-arm64    ./cmd/radiant/
	GOOS=windows GOARCH=amd64 go build -tags light_only $(LDFLAGS) -o dist/radiant-light-windows-amd64.exe ./cmd/radiant/
	@echo "✓ Light release binaries in dist/"

# Smoke test for Light: verifies zero HTTP-LLM symbols (no API key code).
light-smoke: light
	./scripts/smoke-test-light.sh

test:
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -v -count=1

test-short:
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -short

# Test the LIGHT build separately (excludes !light_only files).
test-light:
	CGO_ENABLED=$(CGO_ENABLED) go test -tags light_only ./... -count=1

lint:
	CGO_ENABLED=$(CGO_ENABLED) go vet ./...
	CGO_ENABLED=$(CGO_ENABLED) go vet -tags light_only ./...
	@echo "✓ vet passed (both modes)"

clean:
	rm -rf bin/ dist/

install: build
	cp bin/radiant $(GOPATH)/bin/radiant

# Cross-platform release. All six OS/arch targets the harness
# advertises support for:
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

# Smoke test
smoke: build
	./bin/radiant --version
	./bin/radiant init .tmp-smoke --all --yes
	./bin/radiant validate .tmp-smoke
	rm -rf .tmp-smoke
	@echo "✓ Smoke test passed"

