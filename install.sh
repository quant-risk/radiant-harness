#!/usr/bin/env bash
#
# radiant-harness installer
# -----------------------------------------------------------------------------
# One-shot installer for the radiant-harness CLI.
#
#   curl -fsSL https://raw.githubusercontent.com/quant-risk/radiant-harness/main/install.sh | bash
#
# What it does:
#   1. Detects OS / architecture (linux-amd64, darwin-arm64, …).
#   2. Resolves the latest GitHub release tag (or takes $RADIANT_VERSION).
#   3. Downloads the matching `radiant-<os>-<arch>` binary + SHA256SUMS.
#   4. Verifies the SHA256 of the downloaded binary.
#   5. Installs to $PREFIX/radiant (default /usr/local/bin/radiant).
#   6. (Optional, with --setup-mcp) Runs `radiant setup-mcp` to wire MCP.
#
# Env overrides:
#   RADIANT_VERSION  pin a specific version (e.g. v3.2.7); default = latest
#   PREFIX           install dir; default = /usr/local/bin
#   REPO             override repo (forks); default = quant-risk/radiant-harness
#
# Flags:
#   --setup-mcp      run `radiant setup-mcp` after install
#   --no-verify      skip SHA256 verification (NOT recommended)
#   --dry-run        print what would happen; don't write anything
#
# Exit codes:
#   0 = installed successfully
#   1 = any verification, download, or extraction step failed
#
set -euo pipefail

REPO="${REPO:-quant-risk/radiant-harness}"
PREFIX="${PREFIX:-/usr/local/bin}"
VERSION="${RADIANT_VERSION:-}"
SETUP_MCP=0
VERIFY=1
DRY_RUN=0

# ---- arg parse --------------------------------------------------------------
for arg in "$@"; do
  case "$arg" in
    --setup-mcp) SETUP_MCP=1 ;;
    --no-verify) VERIFY=0 ;;
    --dry-run)   DRY_RUN=1 ;;
    --prefix=*)  PREFIX="${arg#--prefix=}" ;;
    --version=*) VERSION="${arg#--version=}" ;;
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
resolve_latest() {
  local url="https://api.github.com/repos/$REPO/releases/latest"
  local body
  body="$(curl -fsSL "$url")" || bail "failed to fetch latest release from $REPO"
  # tag_name comes first in the JSON; extract the first quoted string.
  echo "$body" | tr -d '\r' | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
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
  install -m 0755 "$TMPDIR/$ASSET" "$PREFIX/radiant" \
    || bail "install failed (does $PREFIX exist and is writable? try PREFIX=~/.local/bin)"
  echo "installed: $($PREFIX/radiant --version 2>&1 || echo "(version unknown)")"
fi

if [ "$SETUP_MCP" = 1 ]; then
  say "wiring MCP into detected host agent"
  "$PREFIX/radiant" setup-mcp
fi

cat <<EOF


  All set. Try:

    radiant --version
    radiant mcp serve --help
    radiant host-info
    radiant setup-mcp    # if not done yet

  Or, to verify the latest MCP possession flow end-to-end, ask any host agent
  that has MCP wired to call: radiant_run(goal="...").

EOF
