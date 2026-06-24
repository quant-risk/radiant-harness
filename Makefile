# Radiant Harness — Makefile
.PHONY: build test lint clean install release

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/radiant ./cmd/radiant/

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short

lint:
	go vet ./...
	@echo "✓ vet passed"

clean:
	rm -rf bin/ dist/

install: build
	cp bin/radiant $(GOPATH)/bin/radiant

# Cross-platform release
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/radiant-linux-amd64 ./cmd/radiant/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/radiant-darwin-arm64 ./cmd/radiant/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/radiant-darwin-amd64 ./cmd/radiant/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/radiant-windows-amd64.exe ./cmd/radiant/
	@echo "✓ Release binaries in dist/"

# Smoke test
smoke: build
	./bin/radiant --version
	./bin/radiant init .tmp-smoke --all --yes
	./bin/radiant validate .tmp-smoke
	rm -rf .tmp-smoke
	@echo "✓ Smoke test passed"
