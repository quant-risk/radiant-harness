# Installation

The Light binary is a single ~7.4 MB executable. No installer, no daemon, no
runtime dependencies, no API key.

## Quick install (recommended)

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@v3.0.1
```

This installs the tagged release to `$GOPATH/bin` (usually `~/go/bin`).
Make sure `$GOPATH/bin` is on your `$PATH`:

```bash
# ~/.zshrc or ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"

# verify
radiant --version       # → radiant 3.0.1
```

## Download a release binary

Pre-built binaries are on the
[releases page](https://github.com/quant-risk/radiant-harness/releases).
Six targets are supported:

| OS | Arch | File |
|----|------|------|
| Linux | amd64 | `radiant-linux-amd64` |
| Linux | arm64 | `radiant-linux-arm64` |
| macOS | amd64 | `radiant-darwin-amd64` |
| macOS | arm64 | `radiant-darwin-arm64` |
| Windows | amd64 | `radiant-windows-amd64.exe` |
| Windows | arm64 | `radiant-windows-arm64.exe` |

### macOS / Linux

```bash
# macOS Apple Silicon
curl -L -o /usr/local/bin/radiant \
  https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-darwin-arm64
chmod +x /usr/local/bin/radiant

# Linux x86_64
curl -L -o /usr/local/bin/radiant \
  https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-linux-amd64
chmod +x /usr/local/bin/radiant

# verify
radiant --version
```

Each release page also publishes a `SHA256SUMS` file you can cross-check:

```bash
curl -L https://github.com/quant-risk/radiant-harness/releases/latest/download/SHA256SUMS | shasum -a 256 -c
```

### Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri "https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-windows-amd64.exe" -OutFile "$env:LOCALAPPDATA\Microsoft\WindowsApps\radiant.exe"
radiant --version
```

## Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/quant-risk/radiant-harness
cd radiant-harness

# build for your current platform
make build
# → ./bin/radiant

# cross-compile all 6 targets
make release
# → bin/radiant-{linux,darwin,windows}-{amd64,arm64}
```

Or with plain `go`:

```bash
CGO_ENABLED=0 go build -o radiant ./cmd/radiant
```

## Wire into your agent

```bash
radiant setup-mcp
```

This auto-detects which agent you have and writes the right config file:

| If you use…           | It writes…                                |
|-----------------------|-------------------------------------------|
| Claude Code           | `.mcp.json`                               |
| Cursor                | `.cursor/mcp.json`                        |
| Windsurf              | `.windsurf/mcp.json`                      |
| Zed                   | `.zed/settings.json`                      |
| VS Code Copilot       | `.vscode/mcp.json`                        |
| OpenAI Codex          | `.codex/config.toml`                      |
| OpenCode              | `.opencode/config.json`                   |
| Hermes                | `.hermes/config.yaml`                     |
| OpenClaw              | `.openclaw/openclaw.json`                 |
| Kimi CLI              | `~/.kimi/mcp.json`                        |
| Cline                 | `~/.cline/mcp.json`                       |

Force a specific agent:

```bash
radiant setup-mcp --agent=claude    # or cursor, codex, hermes, …
radiant setup-mcp --global          # write to ~/.config/<agent>/…
radiant setup-mcp --dry-run         # print the JSON/YAML config that would be written
```

Then **restart your agent**. The next time it sees a non-trivial task, it'll
discover `radiant_run` and use it.

## Verify the install

```bash
radiant host-info
```

Should print something like:

```text
detected agent     : Claude Code
confidence         : 100
signals matched    : CLAUDE_CODE_ENTRYPOINT, CLAUDE_CODE_SHELL_PREFIX
process tree       : /Users/you/.npm/_npx/.../claude (pid 12345)
```

If you're not running inside an agent, `host-info` exits with code 0 and prints
"No agent host detected. radiant-harness is running standalone." That's
expected — `radiant setup-mcp` only needs to write the config; it doesn't
require a live agent process.

To verify the binary is what the project promises:

```bash
# Must return 0 results (no HTTP-LLM symbols):
nm bin/radiant | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
strings bin/radiant | grep -iE 'anthropic|openai|openrouter'

# Binary size:
ls -lh bin/radiant    # ≈ 7.4 MB

# 17/17 checks pass:
make smoke
```

## Updating

```bash
# reinstall the latest
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest

# or re-download the binary
curl -L -o /usr/local/bin/radiant \
  https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
chmod +x /usr/local/bin/radiant
```

To update the wire-up after upgrading:

```bash
radiant setup-mcp    # idempotent — overwrites in place
```

## Uninstalling

```bash
# the binary
rm "$(which radiant)"

# in a project (removes everything radiant wrote)
rm -rf .radiant-harness/

# (only if you also want the harness artifacts gone)
# AGENTS.md, docs/architecture/, docs/product/, specs/ — these are yours, not ours
```

## Troubleshooting

### `radiant: command not found`

`$GOPATH/bin` is not on your `$PATH`. Add it:

```bash
# ~/.zshrc or ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

### `dyld: missing LC_UUID load command` (macOS arm64)

Known issue with Go 1.22.x on Apple Silicon. The Makefile defaults
`CGO_ENABLED=0`; if you built without it, rebuild with:

```bash
CGO_ENABLED=0 go build -o bin/radiant ./cmd/radiant/
```

### `radiant setup-mcp` didn't write anything

`--dry-run` to preview. `--agent=<name>` to force. If you ran without
`--global`, it writes to the project root (where you ran the command),
not your home directory.

### My agent doesn't see `radiant_run`

1. Restart the agent after `radiant setup-mcp` (most agents only read MCP
   config at startup).
2. Run `radiant host-info` from inside the agent — if confidence is 0, the
   agent isn't passing the env vars your client expects. Check
   [`docs/HOST-AGENTS.md`](docs/HOST-AGENTS.md) for the detection matrix.
3. Open the agent's MCP config and confirm the `radiant` server is registered
   and points at the binary path you expect.

### Cross-compile fails on a non-Linux host

The Makefile uses `GOOS` / `GOARCH` env vars. On Windows, use
`make release` from a Git Bash or WSL shell.

## Next

- [README](README.md) — overview
- [EXAMPLES](EXAMPLES.md) — walkthrough: agent calls `radiant_run`
- [Light vs Full](README.md#-light-vs-full) — what's in this binary vs the Full one
- [`docs/TWO-REPOS.md`](docs/TWO-REPOS.md) — rationale for the dual-repo layout