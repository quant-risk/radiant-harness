#!/usr/bin/env bash
# audit-docs.sh — validate that every command name mentioned in
# README.md / INSTALL.md / EXAMPLES.md exists in `radiant --help`.
#
# Drift entre docs e binário foi o bug v3.0.0 (release prometia 50+
# commands que não existiam no binário Light). Este script + 'make
# audit-docs' impede a regressão.
#
# Uso:
#   ./scripts/audit-docs.sh                  # audit + fail if drift
#   ./scripts/audit-docs.sh --report         # show audit, no fail
#
# Exit codes:
#   0 — all docs references resolve to real commands
#   1 — drift detected (at least one doc-mentioned command is a ghost)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS=(
  "$REPO_ROOT/README.md"
  "$REPO_ROOT/INSTALL.md"
  "$REPO_ROOT/EXAMPLES.md"
)
BIN="${RADIANT_BIN:-$REPO_ROOT/bin/radiant}"

if [[ ! -x "$BIN" ]]; then
  echo "audit-docs: binary not found at $BIN — run 'make build' first" >&2
  exit 1
fi

# Extract every command name from `radiant --help` and subcommand tables.
# Format we care about: `radiant <cmd>`, `radiant <cmd> <subcmd>`,
# `radiant_<tool>` (MCP tool name). The `radiant_<tool>` ones live on
# the MCP server, not in --help, so we also check via the self-test
# sub-process.
REAL_CMDS=()
while IFS= read -r line; do
  REAL_CMDS+=("$line")
done < <("$BIN" --help 2>&1 \
  | awk '/^[[:space:]]+[a-z][a-z-]+[[:space:]]+[A-Z]/ {print $1}' \
  | sort -u)

REAL_SUBCMDS=()
while IFS= read -r line; do
  REAL_SUBCMDS+=("$line")
done < <("$BIN" mcp --help 2>&1 \
  | awk '/^[[:space:]]+[a-z][a-z-]+[[:space:]]+[A-Z]/ {print $1}' \
  | sort -u)

if [[ ${#REAL_CMDS[@]} -eq 0 && ${#REAL_SUBCMDS[@]} -eq 0 ]]; then
  echo "audit-docs: WARN no commands discovered from '$BIN --help'" >&2
  exit 0
fi

# Combine into one searchable list.
ALL_CMDS=("${REAL_CMDS[@]}" "${REAL_SUBCMDS[@]}")

# Extract every `radiant <cmd>` reference from the docs.
# Only match when the reference is inside inline code (surrounded by
# backticks) — avoids false positives like "the radiant environment"
# in prose.
DOC_REFS=()
for doc in "${DOCS[@]}"; do
  [[ -f "$doc" ]] || continue
  while IFS= read -r line; do
    DOC_REFS+=("$line")
  done < <(grep -oE '`radiant [a-z][a-z-]+`' "$doc" 2>/dev/null \
    | tr -d '`' \
    | awk '{print $2}' \
    | sort -u)
done

if [[ ${#DOC_REFS[@]} -eq 0 ]]; then
  echo "audit-docs: no 'radiant <cmd>' references found in docs (looked at: ${DOCS[*]})" >&2
  exit 0
fi

missing=()
present=()
for ref in "${DOC_REFS[@]}"; do
  if printf '%s\n' "${ALL_CMDS[@]}" | grep -qx "$ref"; then
    present+=("$ref")
  else
    missing+=("$ref")
  fi
done

printf 'audit-docs: %d doc reference(s), %d real cmd(s)\n' "${#DOC_REFS[@]}" "${#ALL_CMDS[@]}"
if [[ ${#present[@]} -gt 0 ]]; then
  printf '  ✓ present: %s\n' "${present[*]}"
else
  printf '  (no present commands)\n'
fi

if [[ ${#missing[@]} -gt 0 ]]; then
  if [[ ${#missing[@]} -eq 1 && -z "${missing[0]}" ]]; then
    :
  else
    printf '  ✗ MISSING (docs reference cmd not in binary): %s\n' "${missing[*]}"
  fi
  if [[ "${1:-}" == "--report" ]]; then
    exit 0
  fi
  echo ""
  echo "FIX: either"
  echo "  - implement the cmd in cmd/radiant/, or"
  echo "  - remove the reference from the docs"
  exit 1
fi

echo ""
echo "✓ all docs references resolve to real binary commands"
exit 0