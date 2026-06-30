#!/usr/bin/env bash
#
# radiant-harness installer
# -----------------------------------------------------------------------------
# One-shot installer for the radiant-harness CLI.
#
#   curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/main/install.sh | bash -s -- --agent=hermes
#
# What it does:
#   1. Detects OS / architecture (linux-amd64, darwin-arm64, …).
#   2. Resolves the latest GitHub release tag (or takes $RADIANT_VERSION).
#   3. Downloads the matching `radiant-<os>-<arch>` binary + SHA256SUMS.
#   4. Verifies the SHA256 of the downloaded binary.
#   5. Installs to $PREFIX/radiant (default /usr/local/bin/radiant).
#   6. (Optional, with --setup-mcp --agent=<name>) Detects the host agent
#      in this environment and writes the MCP server config (sampling
#      block enabled for Hermes) so the next agent invocation sees the
#      MCP tool without a manual `radiant setup-mcp` step.
#
# Auto-wire (the primary use case):
#   When the user (or the AI agent acting on their behalf) passes
#   --agent=hermes (or claude, codex, cursor, …), the installer:
#     a) Installs the binary,
#     b) Writes the agent's MCP config so the server is registered,
#     c) For Hermes specifically, writes the `sampling:` block so
#        Hermes will respond to sampling/createMessage instead of
#        silently dropping the request.
#     d) Prints the one-line restart instruction.
#
# The agent that receives the task then sees:
#   - `mcp__radiant__possess(task=..., workdir=...)` available as an MCP tool.
# Calling that tool is what triggers harness possession of the agent.
# On hosts with MCP sampling, the harness drives bounded sampling
# round-trips. On hosts without sampling (Codex, for example), it
# switches to self-driven handoff: scaffold + explicit next actions
# for the host agent's native tools.
#
# Env overrides:
#   RADIANT_VERSION  pin a specific version (e.g. v3.3.0); default = latest
#   PREFIX           install dir; default = /usr/local/bin
#   REPO             override repo (forks); default = quant-risk/radiant-harness
#
# Flags:
#   --agent=<name>     auto-wire MCP for this host (claude, codex, hermes,
#                      cursor, minimax, …); pass --setup-mcp alias below
#   --setup-mcp        alias for --agent=<autodetected>; runs setup-mcp
#                      against the detected host
#   --self-for-agent   emit explicit restart + possession instructions at
#                      the end (the canonical "an AI agent just got this
#                      repo" bootstrap). Writes a small instruction file
#                      to the current directory ($WORKDIR, default $PWD)
#                      so the next agent that opens it knows exactly what
#                      to do.
#   --workdir=<path>   override the directory where the agent-instruction
#                      file is written (default: current dir or $PWD).
#   --no-verify        skip SHA256 verification (NOT recommended)
#   --dry-run          print what would happen; don't write anything
#
# Exit codes:
#   0 = installed (and wired, if --agent/--setup-mcp requested) successfully
#   1 = any verification, download, or extraction step failed
#   2 = install succeeded but MCP wiring failed; manual fix instructions printed

set -euo pipefail

REPO="${REPO:-quant-risk/radiant-harness}"
PREFIX="${PREFIX:-/usr/local/bin}"
VERSION="${RADIANT_VERSION:-}"
# Initialised here so `set -u` does not trip on `${AGENT_NAME}` /
# `${SELF_FOR_AGENT}` / `${WORKDIR}` reads further down. Each one is
# filled in by the arg-parse loop above if the user passed it.
AGENT_NAME="${AGENT_NAME:-}"
SELF_FOR_AGENT="${SELF_FOR_AGENT:-0}"
WORKDIR="${WORKDIR:-$PWD}"
SETUP_MCP=0
VERIFY=1
DRY_RUN=0

# ---- arg parse --------------------------------------------------------------
for arg in "$@"; do
  case "$arg" in
    --setup-mcp)      SETUP_MCP=1 ;;
    --agent=*)        AGENT_NAME="${arg#--agent=}" ;;
    --self-for-agent) SELF_FOR_AGENT=1 ;;
    --workdir=*)      WORKDIR="${arg#--workdir=}" ;;
    --no-verify)      VERIFY=0 ;;
    --dry-run)        DRY_RUN=1 ;;
    --prefix=*)       PREFIX="${arg#--prefix=}" ;;
    --version=*)      VERSION="${arg#--version=}" ;;
    -h|--help)
      # Print the docblock at the top of the script.
      awk 'NR>2 && /^set -euo/{exit} {print}' "$0" | sed -n '/^# /p; /^[^#]/q' | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "unknown flag: $arg" >&2; exit 64 ;;
  esac
done

# ---- helpers ----------------------------------------------------------------
bail() { echo "radiant-install: $*" >&2; exit 1; }
say()  { echo "==> $*"; }

run() {
  if [ "$DRY_RUN" = 1 ]; then
    echo "[dry-run] $*"
  else
    eval "$@"
  fi
}

# Resolve latest release tag via GitHub API. Uses curl; no jq, no gh, no xmllint.
#
# Note (v3.7.2): we previously piped the body through `tr -d '\r' |
# grep -m1 '"tag_name"' | sed ...`. That pattern races — `grep -m1`
# closes the pipe after the first match, which sends SIGPIPE to `tr`
# mid-flight on multi-KB JSON bodies under `set -euo pipefail`, and the
# entire script exits with rc=141 before any install step runs.
# Reworked to keep the producer side alive until sed has consumed its
# input. `<<<` here-string also avoids the subshell cost of `echo`.
resolve_latest() {
  local url="https://api.github.com/repos/$REPO/releases/latest"
  local body
  body="$(curl -fsSL "$url")" || bail "failed to fetch latest release from $REPO"
  # tag_name is the first match in the GitHub response; print just the
  # version string. `head -1` so a future response that mentions
  # tag_name more than once does not pass two lines up.
  local tag
  tag="$(grep -m1 '"tag_name"' <<< "$body" \
    | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  if [ -z "$tag" ] || [ "$tag" = "$body" ]; then
    bail "could not parse tag_name from GitHub API response"
  fi
  printf '%s\n' "$tag"
}

# ---- OS / arch detection (BSD + GNU coreutils) ------------------------------
detect_target() {
  local os arch ext
  uname_s="$(uname -s 2>/dev/null || echo unknown)"
  uname_m="$(uname -m 2>/dev/null || echo unknown)"

  case "$uname_s" in
    Linux*)   os=linux ;;
    Darwin*)  os=darwin ;;
    MINGW*|MSYS*|CYGWIN*) os=windows ;;
    *) bail "unsupported OS: $uname_s" ;;
  esac

  case "$uname_m" in
    x86_64|amd64)   arch=amd64 ;;
    aarch64|arm64)  arch=arm64 ;;
    *) bail "unsupported architecture: $uname_m" ;;
  esac

  ext=""
  if [ "$os" = "windows" ]; then ext=".exe"; fi
  echo "${os}-${arch}${ext}"
}

# ---- main -------------------------------------------------------------------

TARGET="$(detect_target)"
say "detected target: $TARGET"

if [ -z "$VERSION" ]; then
  VERSION="$(resolve_latest)"
  say "resolved latest release: $VERSION"
else
  # Normalise: GitHub release tags carry the leading `v` (e.g. v3.7.2).
  # Accept either form so a user passing `3.7.2` or `v3.7.2` both
  # land on the right asset URL. v3.7.2-prep was the documented
  # breakage the audit-install script now catches.
  case "$VERSION" in
    v*) ;;
    *)  VERSION="v$VERSION" ;;
  esac
  say "using pinned version: $VERSION"
fi

BASE="https://github.com/$REPO/releases/download/$VERSION"
ASSET="radiant-$TARGET"
SUMS="SHA256SUMS"

TMPDIR="$(mktemp -d -t radiant-install.XXXXXX)"
trap 'rm -rf "$TMPDIR"' EXIT

say "downloading $ASSET"
DL="$(curl -fsSL -o "$TMPDIR/$ASSET" "$BASE/$ASSET" -w "%{http_code}")" \
  || bail "failed to download $BASE/$ASSET (HTTP $DL)"
ls -l "$TMPDIR/$ASSET" >/dev/null || bail "downloaded file is missing"

say "downloading $SUMS"
curl -fsSL -o "$TMPDIR/$SUMS" "$BASE/$SUMS" \
  || bail "failed to download $SUMS (cannot verify)"

if [ "$VERIFY" = 1 ]; then
  say "verifying SHA256"
  EXPECTED="$(grep -E "[[:space:]]$(basename "$ASSET")\$" "$TMPDIR/$SUMS" | awk '{print $1}')"
  if [ -z "$EXPECTED" ]; then
    bail "no checksum found for $ASSET in $SUMS"
  fi
  ACTUAL=""
  if command -v shasum >/dev/null 2>&1; then
    ACTUAL="$(shasum -a 256 "$TMPDIR/$ASSET" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "$TMPDIR/$ASSET" | awk '{print $1}')"
  else
    bail "no sha256 tool found (need shasum or sha256sum)"
  fi
  if [ "$ACTUAL" != "$EXPECTED" ]; then
    bail "checksum mismatch:
  expected: $EXPECTED
  actual:   $ACTUAL"
  fi
  say "SHA256 OK"
fi

say "installing to $PREFIX/radiant"
if [ "$DRY_RUN" = 1 ]; then
  echo "[dry-run] install -m 0755 $TMPDIR/$ASSET $PREFIX/radiant"
else
  # Create the parent directory if missing (PREFIX=~/.local/bin or other
  # custom paths may not exist on first install; /usr/local/bin typically
  # does, but we don't assume).
  if [ ! -d "$PREFIX" ]; then
    mkdir -p "$PREFIX" 2>/dev/null \
      || bail "could not create $PREFIX (try PREFIX=\$HOME/.local/bin and rerun)"
  fi
  install -m 0755 "$TMPDIR/$ASSET" "$PREFIX/radiant" \
    || bail "install failed (does $PREFIX exist and is writable? try PREFIX=~/.local/bin)"
  echo "installed: $($PREFIX/radiant --version 2>&1 || echo "(version unknown)")"
fi

if [ "$SETUP_MCP" = 1 ] || [ -n "$AGENT_NAME" ]; then
  # Resolve which agent to wire. Prefer explicit --agent=NAME; fall
  # back to auto-detect via `radiant host-info`. If neither yields, the
  # user can still wire manually after the agent is first launched.
  if [ -n "$AGENT_NAME" ]; then
    TARGET_AGENT="$AGENT_NAME"
  else
    HOST_OUT="$("$PREFIX/radiant" host-info 2>/dev/null || true)"
    TARGET_AGENT="$(printf '%s\n' "$HOST_OUT" | grep -m1 'detected agent' | awk -F: '{print $2}' | tr -d ' ')"
  fi

  if [ -n "$TARGET_AGENT" ]; then
    say "wiring MCP for host: $TARGET_AGENT"
    WIRE_ERR=$("$PREFIX/radiant" setup-mcp --agent="$TARGET_AGENT" --global 2>&1) || true
    WIRE_RC=$?
    if [ "$WIRE_RC" -ne 0 ]; then
      echo ""
      echo "WARNING: MCP wiring for $TARGET_AGENT failed (rc=$WIRE_RC)."
      echo ""
      echo "$WIRE_ERR" | head -20
      echo ""
      echo "Retry manually after the host is launched:"
      echo "  $PREFIX/radiant setup-mcp --agent=$TARGET_AGENT --global"
      echo "  $PREFIX/radiant doctor --mcp"
      exit 2
    fi
  else
    say "no host agent auto-detected; skipping MCP wiring"
    say "after the agent launches, run: $PREFIX/radiant setup-mcp"
  fi
fi

cat <<EOF


  All set. Try:

    $PREFIX/radiant --version
    $PREFIX/radiant mcp self-test     # verify MCP wire-up
    $PREFIX/radiant doctor --mcp      # verify host agent config

  If you ran --agent=<name> (or --setup-mcp), the host's MCP config is
  already updated. Restart the agent to pick up the new MCP server, then
  call the possession tool:

    mcp__radiant__possess(task="<the goal>", workdir="<cwd>")

  The harness takes over. If the host supports sampling/createMessage,
  it drives bounded sampling round-trips. If not, it returns a
  Self-driven handoff with the spec dir, files to update, verification
  command, and remaining [host-agent: fill in] markers.

EOF

# ---- Self-bootstrap for the host agent --------------------------------------
# When --self-for-agent is set, write a machine-readable instruction file to
# the current working directory (or $WORKDIR) so the *next* agent that opens
# this directory sees: "radiant is installed; here is exactly what to do."
# This side-steps the chicken-and-egg problem where the agent hasn't loaded
# the new MCP config yet and might run cli commands instead.
if [ "$SELF_FOR_AGENT" = 1 ]; then
  if [ -z "$WORKDIR" ]; then
    WORKDIR="${WORKDIR:-$PWD}"
    WORKDIR="$(cd "$WORKDIR" 2>/dev/null && pwd || echo "$WORKDIR")"
  fi
  mkdir -p "$WORKDIR/.radiant-harness" 2>/dev/null || true
  INSTALLED_BIN="$PREFIX/radiant"
  RST=""
  case "${TARGET_AGENT:-}" in
    hermes)        RST="send /reload-mcp in this chat" ;;
    claude)        RST="restart the shell session that runs `claude`" ;;
    codex)         RST="re-launch codex (exit the current and run again)" ;;
    cursor)        RST="reopen the Cursor window" ;;
    opencode)      RST="reopen the OpenCode editor" ;;
    windsurf)      RST="reopen Windsurf" ;;
    vscode-copilot) RST="reload the VS Code window (Command Palette > 'Reload Window')" ;;
    zed)           RST="restart Zed" ;;
    *)             RST="restart this agent (MCP servers are loaded at process start)" ;;
  esac
  cat > "$WORKDIR/.radiant-harness/INIT.json" 2>/dev/null <<JSON
{
  "installed_at":    "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "radiant_binary":  "$INSTALLED_BIN",
  "version":         "$VERSION",
  "host_agent":      "${TARGET_AGENT:-auto}",
  "restart_hint":    "$RST",
  "next_call": {
    "tool":        "mcp__radiant__possess",
    "task_arg":    "<the user's original prompt, verbatim>",
    "workdir_arg": "$WORKDIR",
    "profile_arg": "standard"
  }
}
JSON
  cat > "$WORKDIR/.radiant-harness/AGENTS.md" 2>/dev/null <<MD
# AGENTS.md (auto-written by radiant-harness installer)

This project has **radiant-harness** installed at:
\`$INSTALLED_BIN\`

The host agent (likely **${TARGET_AGENT:-auto}**) already has its MCP
config pointing at that binary. The next step is to reload:

  > $RST

After reloading, **call the MCP tool** with the user's original prompt:

  mcp__radiant__possess(
      task     = "<user's original prompt, verbatim>",
      workdir  = "$WORKDIR",
      profile  = "standard"
  )

Do **not** run \`radiant loop\`, \`radiant run\`, \`radiant fleet\`,
\`radiant model\`, \`radiant profile\`, \`radiant evaluate\`, or any
other CLI subcommand directly — those are gated behind
\`RADIANT_INTERNAL=1\` and the host agent must own the task via the
MCP tool above.
MD
  cat > "$WORKDIR/.radiant-harness/NEXT.txt" 2>/dev/null <<TXT
=== NEXT STEP for the AI agent in this session ===

Restart yourself: $RST

Then call:
  mcp__radiant__possess(
      task    = "<the user's original prompt>",
      workdir = "$WORKDIR",
      profile = "standard"
  )

The harness takes over via sampling/createMessage when available, or
returns a Self-driven handoff when this host does not implement sampling.
TXT
  say "agent-bootstrap files written to: $WORKDIR/.radiant-harness/"
  say "  - AGENTS.md    (the contract)"
  say "  - INIT.json    (machine-readable)"
  say "  - NEXT.txt     (next-step prompt)"
  say ""
  say "NEXT STEP for the agent in this directory:"
  say "  $RST"
  say ""
fi
