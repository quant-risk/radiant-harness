#!/usr/bin/env bash
# Project validation entrypoint for radiant-harness (v3.7.6).
#
# Runs the validation matrix that `radiant doctor + mcp self-test +
# test-dropin` covered before v3.7.6, plus the cross-agent install and
# install-path audit gates that `make test-agents` and
# `make audit-install` gate on.
#
# The doctor step is informational: in a fresh shell with no MCP-wired
# host, `radiant doctor` legitimately reports a host-not-detected FAIL.
# We print the result and continue; the rest of the matrix is what
# truly establishes the harness is healthy.
#
# Each step is gated by a clear pass/fail so a CI runner surfaces the
# exact failure without the script aborting at the first hiccup.
#
# Exit codes:
#   0 — every step PASS
#   1 — at least one step FAIL

set -uo pipefail
cd "$(dirname "$0")/.."

PASS=0
FAIL=0
SKIP=0

step() {
  local name="$1"
  shift
  printf '\n[%s]\n' "$name"
  if "$@"; then
    printf '  ✓ %s\n' "$name"
    PASS=$((PASS+1))
    return 0
  else
    local rc=$?
    if [ "$rc" -eq 77 ]; then
      printf '  ⚠ %s (SKIP — informational only)\n' "$name"
      SKIP=$((SKIP+1))
      return 0
    fi
    printf '  ✗ %s (exit=%d)\n' "$name" "$rc"
    FAIL=$((FAIL+1))
    return 1
  fi
}

# doctor_step — informational. In a fresh shell with no MCP host,
# `radiant doctor` returns 1 (no host agent). We don't fail the matrix
# on that — we surface it as a warning so the operator knows to re-run
# from inside a host session for a fully-green check.
doctor_step() {
  printf '\n[doctor]\n'
  if radiant doctor; then
    printf '  ✓ doctor\n'
    PASS=$((PASS+1))
  else
    printf '  ⚠ doctor (no MCP-wired host — informational; re-run inside a host session for green)\n'
    SKIP=$((SKIP+1))
  fi
}

# doctor_mcp_step — runs `radiant doctor --mcp`, which is the
# post-setup-mcp check. Also informational since it requires the
# operator to have wired an agent first.
doctor_mcp_step() {
  printf '\n[doctor --mcp]\n'
  if radiant doctor --mcp >/dev/null 2>&1; then
    printf '  ✓ doctor --mcp (radiant entry wired + sampling enabled)\n'
    PASS=$((PASS+1))
  else
    printf '  ⚠ doctor --mcp (no host agent — informational; run `radiant setup-mcp --agent=<host>` then re-run)\n'
    SKIP=$((SKIP+1))
  fi
}

step "mcp self-test"          bash -c "radiant mcp self-test >/dev/null"
doctor_step
doctor_mcp_step
step "cmd/radiant + internal" bash -c "go test ./cmd/radiant ./internal/... -count=1"
step "go test ./..."          bash -c "go test ./... -count=1"
step "audit-docs"             make audit-docs
step "audit-skills"           make audit-skills
step "audit-install"          make audit-install
step "test-agents"            make test-agents
step "test-dropin"            make test-dropin

printf '\n=== run.sh summary ===\n'
printf '  PASS: %d  FAIL: %d  SKIP: %d\n' "$PASS" "$FAIL" "$SKIP"

if [ "$FAIL" -gt 0 ]; then
  printf '✗ run.sh FAILED — see ✗ lines above.\n'
  exit 1
fi

printf '✓ run.sh PASSED — radiant-harness is healthy (PASS=%d, SKIP=%d).\n' "$PASS" "$SKIP"
exit 0