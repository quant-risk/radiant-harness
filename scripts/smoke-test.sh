#!/usr/bin/env bash
#
# Smoke test for the radiant binary.
#
# Verifies the three core properties at the artifact level:
#   1. Binary compiles.
#   2. Binary has NO HTTP-LLM symbols (chatAnthropic, HTTPBackend, api.*).
#   3. The binary self-reports its version as 3.2.0.
#
# Usage:
#   scripts/smoke-test.sh                          # build + test
#   BIN=/path/to/radiant scripts/smoke-test.sh     # test existing
#
# Exit code 0 = all checks pass. Non-zero on first failure.

set -euo pipefail

# Resolve repo root (script lives in scripts/, parent is repo root).
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

BIN="${BIN:-}"
case "$BIN" in
  "")
    echo "==> building radiant"
    mkdir -p bin
    BIN="bin/radiant"
    VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
    CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$VERSION" -o "$BIN" ./cmd/radiant
    ;;
  /*) ;;
  *) BIN="$REPO_ROOT/$BIN" ;;
esac

red()   { printf '\033[31m%s\033[0m' "$*"; }
green() { printf '\033[32m%s\033[0m' "$*" >&2; }

fail() { echo "$(red FAIL): $*" >&2; exit 1; }
ok()   { echo "$(green OK): $*"; }

# 1. Binary exists and is executable.
[ -x "$BIN" ] || fail "$BIN is missing or not executable"
ok "$BIN exists and is executable"

# 2. Version contains 3.3.x (any 3.3 release).
V="$("$BIN" --version 2>&1 || true)"
case "$V" in
  *"3.3.0"*|*"3.3.1"*|*"3.3.2"*) ok "version reports '$V'" ;;
  *)                           fail "expected version to contain '3.3.0', '3.3.1', or '3.3.2', got: $V" ;;
esac

# 3. NO HTTP-LLM symbols.
HTTP_LLM_SYMBOLS=""
if command -v nm >/dev/null 2>&1; then
  HTTP_LLM_SYMBOLS="$(nm "$BIN" 2>/dev/null | grep -iE 'chatAnthropic|^[^ ]+ T .*HTTPBackend|^[^ ]+ T .*NewHTTPBackend' || true)"
fi
if [ -z "$HTTP_LLM_SYMBOLS" ] && command -v strings >/dev/null 2>&1; then
  HTTP_LLM_SYMBOLS="$(strings "$BIN" | grep -iE 'chatAnthropic|api\.anthropic\.com|api\.openai\.com|openrouter\.ai' || true)"
fi

if [ -n "$HTTP_LLM_SYMBOLS" ]; then
  fail "binary contains HTTP-LLM symbols/strings:\n$HTTP_LLM_SYMBOLS"
fi
ok "no HTTP-LLM symbols in $BIN"

# 4. No env var instructions about API keys.
API_HINTS="$("$BIN" --help 2>&1 | grep -iE 'API_KEY|openai|anthropic|openrouter' || true)"
if [ -n "$API_HINTS" ]; then
  fail "binary --help contains API key references:\n$API_HINTS"
fi
ok "no API key references in $BIN --help"

# 5. Available commands include setup-mcp, mcp serve, host-info.
HELP="$("$BIN" --help 2>&1 || true)"
for cmd in setup-mcp mcp host-info; do
  case "$HELP" in
    *"$cmd"*) ok "command '$cmd' present" ;;
    *)        fail "command '$cmd' missing from --help" ;;
  esac
done

# 6. setup-mcp --help mentions the supported agents.
SETUP_HELP="$("$BIN" setup-mcp --help 2>&1 || true)"
for agent in claude cursor codex hermes kimi openclaw cline opencode; do
  case "$SETUP_HELP" in
    *"$agent"*) ok "setup-mcp mentions '$agent'" ;;
    *)          fail "setup-mcp --help missing '$agent' agent" ;;
  esac
done

# 7. host-info runs and produces output.
HOST_INFO_OUT="$("$BIN" host-info 2>&1 || true)"
case "$HOST_INFO_OUT" in
  *"Detected host agent"*) ok "host-info emitted status line" ;;
  *)                       fail "host-info produced no status line:\n$HOST_INFO_OUT" ;;
esac

# 8. Binary ≤ 15 MB.
BYTES=$(stat -f%z "$BIN" 2>/dev/null || stat -c%s "$BIN" 2>/dev/null)
MAX=$((15 * 1024 * 1024))
[ "$BYTES" -le "$MAX" ] || fail "binary is $BYTES bytes, > 15 MB"
ok "binary size: $BYTES bytes (≤ 15 MB)"

echo
green "All smoke checks passed for $BIN"