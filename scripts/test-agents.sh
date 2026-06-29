#!/usr/bin/env bash
#
# scripts/test-agents.sh — cross-agent install/validation matrix
# -----------------------------------------------------
# For each of the 12 host agents supported by radiant-harness, simulate
# the agent's environment (env vars + sandbox HOME), run setup-mcp,
# then run doctor --mcp + mcp self-test. Emits PASS/FAIL per agent
# and writes a Markdown status matrix to .radiant-harness/agent-matrix.md.
#
# Manual invocation. No cron, no polling, no daemon.
#
# Usage:
#   scripts/test-agents.sh                    # run all 12 agents
#   scripts/test-agents.sh one <agent>        # run a single agent (debug)
#   scripts/test-agents.sh --json            # JSON output instead of plain text
#   RADIANT=path/to/radiant scripts/test-agents.sh  # override binary
#
# Exit code: 0 if all agents PASS, 1 if any FAIL.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MATRIX_JSON="${REPO_ROOT}/scripts/host-agent-matrix.json"
REPORT_DIR="${REPO_ROOT}/.radiant-harness"
REPORT_MD="${REPORT_DIR}/agent-matrix.md"

RADIANT="${RADIANT:-${REPO_ROOT}/bin/radiant}"
JSON_OUT=0
ONE_AGENT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --json)  JSON_OUT=1 ;;
    one)
      shift
      ONE_AGENT="$1"
      ;;
    *)       echo "unknown flag: $1" >&2; exit 64 ;;
  esac
  shift
done

if [ ! -x "$RADIANT" ]; then
  echo "radiant binary not found at $RADIANT — run 'make build' first" >&2
  exit 1
fi

if [ ! -f "$MATRIX_JSON" ]; then
  echo "matrix file missing: $MATRIX_JSON" >&2
  exit 1
fi

mkdir -p "$REPORT_DIR"

PASS=0
FAIL=0
RESULTS=()

# run_agent runs setup-mcp + doctor + self-test against a sandbox.
# Two sandbox subtrees are used:
#   $sandbox/Home/   → user-level config writers (claude/codex/hermes/etc)
#   $sandbox/proj/   → project-level writers (cursor/windsurf/zed/vscode
#                      hardcode cwd in cmd_setup_mcp.go and ignore --global)
# Caller is responsible for exporting the agent's env vars and clearing
# them on the way out.
run_agent() {
  local name="$1" cfg_relpath="$2"
  local sandbox; sandbox="$(mktemp -d -t radiant-agent-XXXXXX)"
  local sandbox_home="$sandbox/Home"
  local sandbox_proj="$sandbox/proj"
  mkdir -p "$sandbox_home/.radiant-harness" "$sandbox_proj"

  (
    cd "$sandbox_proj" && \
    HOME="$sandbox_home" \
    RADIANT_HARNESS_AGENT="$name" \
    "$RADIANT" setup-mcp --agent="$name" --global --force \
      >"/tmp/setup-mcp-${name}.log" 2>&1
  ) || local rc=$?
  local setup_rc=${rc:-0}
  rc=0

  local doctor_out
  doctor_out="$(cd "$sandbox_proj" && HOME="$sandbox_home" "$RADIANT" doctor --mcp 2>&1)" || true
  local doctor_verdict
  doctor_verdict="$(printf '%s\n' "$doctor_out" | grep -oE 'verdict[[:space:]]*=[[:space:]]*[A-Z]+' | awk '{print $NF}' | head -1)"
  [ -z "$doctor_verdict" ] && doctor_verdict="UNKNOWN"

  local selftest
  selftest="$(cd "$sandbox_proj" && HOME="$sandbox_home" "$RADIANT" mcp self-test 2>&1)" || true
  local selftest_pass="FAIL"
  printf '%s' "$selftest" | grep -q 'self-test: PASS' && selftest_pass="PASS"

  # cfg is "present" if EITHER the user-level or the project-level path
  # exists. This is the symptom-finder's purpose: regardless of which
  # cmd_setup_mcp.go branch won, the matrix sees what landed.
  local cfg_path_home="$sandbox_home/$cfg_relpath"
  local cfg_path_proj="$sandbox_proj/$cfg_relpath"
  local cfg_present="MISSING"
  local cfg_where="—"
  if [ -f "$cfg_path_home" ]; then cfg_present="OK"; cfg_where="home"; fi
  if [ -f "$cfg_path_proj" ]; then cfg_present="OK"; cfg_where="$cfg_where/proj"; fi

  local verdict="FAIL"
  if [ "$doctor_verdict" = "OK" ] && [ "$cfg_present" = "OK" ] && [ "$selftest_pass" = "PASS" ]; then
    verdict="PASS"
  fi

  rm -rf "$sandbox"

  RESULTS+=("{\"name\":\"$name\",\"verdict\":\"$verdict\",\"setup_rc\":$setup_rc,\"cfg_file\":\"$cfg_relpath\",\"cfg_where\":\"$cfg_where\",\"cfg_present\":\"$cfg_present\",\"doctor_verdict\":\"$doctor_verdict\",\"self_test\":\"$selftest_pass\"}")
}

# Build the per-agent bash script via python (env vars can contain
# spaces, paths, etc. — emit them as bash, parse once at source-time).
ONE_AGENT_FILTER="$ONE_AGENT" python3 - "$MATRIX_JSON" >/tmp/agent-entries.sh <<'PY'
import json, os, shlex, sys
filter_name = os.environ.get("ONE_AGENT_FILTER", "")
data = json.load(open(sys.argv[1]))
all_keys = sorted({k for a in data["agents"] for k in a["env"].keys()})
unset_lines = "\n".join(f"unset {shlex.quote(k)}" for k in all_keys)
for a in data["agents"]:
    name = a["name"]
    cfg = a.get("config_file_relpath_global") or a.get("config_file_relpath") or a["config_file_relpath_project"]
    envs = "\n".join(f"export {shlex.quote(k)}={shlex.quote(v)}" for k, v in a["env"].items())
    print(
        "# --- agent: " + name + " ---\n"
        + unset_lines + "\n"
        + envs + "\n"
        + ('if [ -z "' + filter_name + '" ] || [ "' + filter_name + '" = "' + name + '" ]; then\n'
           '  run_agent "' + name + '" "' + cfg + '"\n'
           'fi\n')
    )
PY

# Source the generated script. Environment variables set/unset by the
# generated code apply to this shell, so the unset prelude before each
# agent block cleanly drops prior agent state.
. /tmp/agent-entries.sh

# Render the Markdown matrix.
{
  echo "# Cross-agent install matrix"
  echo
  echo "_Last run: $(date -u +%Y-%m-%dT%H:%M:%SZ)_"
  echo
  echo "| agent          | config file                       | cfg-where | cfg-present | doctor --mcp | mcp self-test |"
  echo "|----------------|-----------------------------------|-----------|-------------|--------------|---------------|"
  for r in "${RESULTS[@]}"; do
    name="$(printf '%s' "$r" | sed -E 's/.*"name":"([^"]+)".*/\1/')"
    cfg="$(printf '%s' "$r" | sed -E 's/.*"cfg_file":"([^"]+)".*/\1/')"
    cfg_w="$(printf '%s' "$r" | sed -E 's/.*"cfg_where":"([^"]+)".*/\1/')"
    cfg_p="$(printf '%s' "$r" | sed -E 's/.*"cfg_present":"([^"]+)".*/\1/')"
    doctor="$(printf '%s' "$r" | sed -E 's/.*"doctor_verdict":"([^"]+)".*/\1/')"
    self_t="$(printf '%s' "$r" | sed -E 's/.*"self_test":"([^"]+)".*/\1/')"
    case "$doctor" in
      OK) dc="✓ " ;;
      *)  dc="⚠️ " ;;
    esac
    case "$cfg_p" in
      OK) cp_ok="✓ " ;;
      *)  cp_ok="⚠️ " ;;
    esac
    case "$self_t" in
      PASS) st_ok="✓ " ;;
      *)    st_ok="⚠️ " ;;
    esac
    printf "| %-14s | %-33s | %-9s | %s%-11s | %s%-10s | %s%-13s |\n" \
      "$name" "$cfg" "$cfg_w" "$cp_ok" "$cfg_p" "$dc" "$doctor" "$st_ok" "$self_t"
  done
  echo
  total="?"
  if [ ${#RESULTS[@]} -gt 0 ]; then
    total="${#RESULTS[@]}"
  fi
  passed=$(printf '%s\n' "${RESULTS[@]}" | grep -c '"verdict":"PASS"' || true)
  failed=$((total - passed))
  echo "_Total: $total agents; $passed PASS, $failed FAIL._"
  echo
  echo "Generated by \`scripts/test-agents.sh\` (Sprint 5)."
} > "$REPORT_MD"

if [ "$JSON_OUT" = 1 ]; then
  printf '{"agents":[\n'
  for i in "${!RESULTS[@]}"; do
    if [ "$i" -gt 0 ]; then printf ',\n'; fi
    printf '%s' "${RESULTS[$i]}"
  done
  printf '\n]}\n'
  exit 0
fi

echo ""
echo "matrix written: $REPORT_MD"
# Exit non-zero if any FAIL.
for r in "${RESULTS[@]}"; do
  if ! printf '%s' "$r" | grep -q '"verdict":"PASS"'; then
    exit 1
  fi
done
exit 0
