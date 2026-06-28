#!/usr/bin/env bash
# radiant installer — detects OS/arch, installs binary, optionally sets up MCP
set -euo pipefail

REPO="quant-risk/radiant-harness"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="radiant"
VERSION="${RADIANT_VERSION:-latest}"

# ── detect platform ──────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

ASSET="radiant-${OS}-${ARCH}"
[ "$OS" = "windows" ] && ASSET="${ASSET}.exe"

echo "radiant installer"
echo "  Platform : ${OS}/${ARCH}"
echo "  Target   : ${INSTALL_DIR}/${BIN_NAME}"
echo ""

# ── resolve version ──────────────────────────────────────────────────────────
if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

if [ -z "$VERSION" ]; then
  echo "Could not resolve latest version. Set RADIANT_VERSION=vX.Y.Z to pin." >&2
  exit 1
fi

echo "  Version  : ${VERSION}"
echo ""

# ── download ─────────────────────────────────────────────────────────────────
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

echo "Downloading ${URL} ..."
if command -v curl &>/dev/null; then
  curl -fsSL "$URL" -o "$TMP"
elif command -v wget &>/dev/null; then
  wget -qO "$TMP" "$URL"
else
  echo "curl or wget required" >&2
  exit 1
fi

chmod +x "$TMP"

# ── verify binary works ───────────────────────────────────────────────────────
if ! "$TMP" --version &>/dev/null; then
  echo "Downloaded binary failed to run — architecture mismatch?" >&2
  exit 1
fi

# ── install ───────────────────────────────────────────────────────────────────
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BIN_NAME}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "$TMP" "${INSTALL_DIR}/${BIN_NAME}"
fi

echo "✓ radiant installed to ${INSTALL_DIR}/${BIN_NAME}"
echo "  Version: $("${INSTALL_DIR}/${BIN_NAME}" --version)"
echo ""

# ── post-install hint ─────────────────────────────────────────────────────────
echo "Next step — register the MCP server with your agent:"
echo ""
echo "  radiant setup-mcp              # auto-detect agent"
echo "  radiant setup-mcp --agent=claude"
echo "  radiant setup-mcp --agent=cursor"
echo "  radiant setup-mcp --global    # write to ~/.claude/settings.json"
echo ""
echo "After that, any prompt works:"
echo "  \"use radiant-harness to: <your goal>\""
