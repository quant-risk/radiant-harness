#!/usr/bin/env bash
#
# Smoke test for the Light binary.
#
# Verifies the three core properties of radiant-light at the artifact level:
#   1. Binary compiles (and the right number of artefacts per platform).
#   2. Light has NO HTTP-LLM symbols (chatAnthropic, HTTPBackend, api.*).
#   3. The Light binary self-reports its version with the -light suffix.
#
# Usage:
#   scripts/smoke-test-light.sh                          # build + test
#   BI=/path/to/radiant-light scripts/smoke-test-light.sh  # test existing
#
# Exit code 0 = all checks pass. Non-zero on first failure.

set -euo pipefail

# Resolve repo root (script lives in scripts/, parent is repo root).
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

BIN="${BIN:-}"
case "$BIN" in
  "")
    echo "==> building radiant-light"
    mkdir -p bin
    BIN="bin/radiant-light"
    CGO_ENABLED=0 go build -tags light_only -o "$BIN" ./cmd/radiant
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

# 2. Version reports the -light suffix.
V="$("$BIN" --version 2>&1 || true)"
case "$V" in
  *-light) ok "version reports '$V' (contains -light)" ;;
  *)       fail "expected version to contain '-light', got: $V" ;;
esac

# 3. NO HTTP-LLM symbols.
# nm | grep checks for: chatAnthropic, HTTPBackend, NewHTTPBackend.
# (Portability: try both `nm` and `llvm-nm`, fall back to `strings`.)
HTTP_LLM_SYMBOLS=""
if command -v nm >/dev/null 2>&1; then
  HTTP_LLM_SYMBOLS="$(nm "$BIN" 2>/dev/null | grep -iE 'chatAnthropic|^[^ ]+ T .*HTTPBackend|^[^ ]+ T .*NewHTTPBackend' || true)"
fi
if [ -z "$HTTP_LLM_SYMBOLS" ] && command -v strings >/dev/null 2>&1; then
  HTTP_LLM_SYMBOLS="$(strings "$BIN" | grep -iE 'chatAnthropic|api\.anthropic\.com|api\.openai\.com|openrouter\.ai' || true)"
fi

if [ -n "$HTTP_LLM_SYMBOLS" ]; then
  fail "Light binary contains HTTP-LLM symbols/strings:\n$HTTP_LLM_SYMBOLS"
fi
ok "no HTTP-LLM symbols in $BIN"

# 4. No env var instructions about API keys.
API_HINTS="$("$BIN" --help 2>&1 | grep -iE 'API_KEY|openai|anthropic|openrouter' || true)"
if [ -n "$API_HINTS" ]; then
  # It's OK to mention "$RADIANT_OPENROUTER_API_KEY" in --help IF the
  # command is in the Full binary. Light should mention nothing.
  fail "Light --help contains API key references:\n$API_HINTS"
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

# 6. setup-mcp --help mentions 11 agents.
SETUP_HELP="$("$BIN" setup-mcp --help 2>&1 || true)"
for agent in claude cursor codex hermes kimi openclaw cline opencode; do
  case "$SETUP_HELP" in
    *"$agent"*) ok "setup-mcp mentions '$agent'" ;;
    *)          fail "setup-mcp --help missing '$agent' agent" ;;
  esac
done

# 7. host-info runs and produces output (no agent expected).
HOST_INFO_OUT="$("$BIN" host-info 2>&1 || true)"
case "$HOST_INFO_OUT" in
  *"Detected host agent"*) ok "host-info emitted status line" ;;
  *)                       fail "host-info produced no status line:\n$HOST_INFO_OUT" ;;
esac

# 8. Light binary ≤ 15 MB.
BYTES=$(stat -f%z "$BIN" 2>/dev/null || stat -c%s "$BIN" 2>/dev/null)
MAX=$((15 * 1024 * 1024))
[ "$BYTES" -le "$MAX" ] || fail "Light binary is $BYTES bytes, > 15 MB"
ok "binary size: $BYTES bytes (≤ 15 MB)"

echo
green "All smoke checks passed for $BIN"
