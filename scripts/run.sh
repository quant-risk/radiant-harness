#!/usr/bin/env bash
# Project validation entrypoint for radiant-harness.

set -euo pipefail
cd "$(dirname "$0")/.."

echo "[radiant] doctor"
radiant doctor

echo "[radiant] mcp self-test"
radiant mcp self-test

echo "[radiant] targeted tests"
go test ./cmd/radiant ./internal/...

echo "[radiant] drop-in E2E"
make test-dropin

echo "[radiant] validation complete."
