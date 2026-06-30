#!/usr/bin/env bash
# audit-skills.sh — validate that every skill referenced in
# selfDrivenSkillHints (cmd/radiant/cmd_mcp_possess_self_driven.go) exists
# as a directory under internal/skill/skills/<name>/.
#
# Drift entre código e bundle foi o bug v3.7.1 hot-fix (4/13 hints
# apontavam pra skills inexistentes: credit-risk-modeling, risk-management,
# ml-modeling, regulatory-compliance). Este script + `make audit-skills`
# impede a regressão.
#
# Uso:
#   ./scripts/audit-skills.sh                       # audit + fail if drift
#   ./scripts/audit-skills.sh --report              # show audit, no fail
#
# Exit codes:
#   0 — all hints reference real bundled skills
#   1 — drift detected (at least one hint is a ghost)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HINTS_FILE="$REPO_ROOT/cmd/radiant/cmd_mcp_possess_self_driven.go"
SKILLS_DIR="$REPO_ROOT/internal/skill/skills"

if [[ ! -f "$HINTS_FILE" ]]; then
  echo "audit-skills: hints file not found: $HINTS_FILE" >&2
  exit 1
fi
if [[ ! -d "$SKILLS_DIR" ]]; then
  echo "audit-skills: skills dir not found: $SKILLS_DIR" >&2
  exit 1
fi

# Extract every {keyword, skill-name} pair from the hint map. The map
# entries look like `{"credit", "credit-risk"},`.
HINT_SKILLS=()
while IFS= read -r line; do
  HINT_SKILLS+=("$line")
done < <(grep -oE '\{"[a-z-]+", "[a-z-]+"\}' "$HINTS_FILE" \
  | sed -E 's/.*"([a-z-]+)"\}$/\1/' \
  | sort -u)

if [[ ${#HINT_SKILLS[@]} -eq 0 ]]; then
  echo "audit-skills: WARN no hint entries found in $HINTS_FILE" >&2
  exit 0
fi

BUNDLE_SKILLS=()
for d in "$SKILLS_DIR"/*/; do
  name="$(basename "$d")"
  if [[ -f "$d/SKILL.md" ]]; then
    BUNDLE_SKILLS+=("$name")
  fi
done

missing=()
present=()
for s in "${HINT_SKILLS[@]}"; do
  if printf '%s\n' "${BUNDLE_SKILLS[@]}" | grep -qx "$s"; then
    present+=("$s")
  else
    missing+=("$s")
  fi
done

printf 'audit-skills: %d hint(s), %d bundled skill(s)\n' "${#HINT_SKILLS[@]}" "${#BUNDLE_SKILLS[@]}"
printf '  ✓ present: %s\n' "${present[*]}"

if [[ ${#missing[@]} -gt 0 ]]; then
  printf '  ✗ MISSING (hint map references skill not in bundle): %s\n' "${missing[*]}"
  if [[ "${1:-}" == "--report" ]]; then
    exit 0
  fi
  echo ""
  echo "FIX: either"
  echo "  - rename the hint in cmd_mcp_possess_self_driven.go to a real skill, or"
  echo "  - add the skill under internal/skill/skills/$missing/SKILL.md"
  exit 1
fi

echo ""
echo "✓ all hint map entries reference real bundled skills"
exit 0