# Installation

## Quick install (recommended)

```bash
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest
```

This installs the latest tagged release to `$GOPATH/bin` (usually
`~/go/bin`). Make sure `$GOPATH/bin` is on your `$PATH`.

```bash
# verify
radiant --version
```

## Download a release binary

Pre-built binaries are available on the
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
# download
curl -L -o radiant https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-darwin-arm64
chmod +x radiant

# install
sudo mv radiant /usr/local/bin/

# verify
radiant --version
```

### Windows

```powershell
# download
Invoke-WebRequest -Uri "https://github.com/quant-risk/radiant-harness/releases/latest/download/radiant-windows-amd64.exe" -OutFile "radiant.exe"

# install (PowerShell as admin)
Move-Item radiant.exe $env:ProgramFiles\Radiant\

# add to PATH (Settings → System → Environment Variables)

# verify
radiant --version
```

## Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/quant-risk/radiant-harness.git
cd radiant-harness

# build for your current platform
make build

# or cross-compile all 6 targets
make release
# outputs to dist/
```

## Configure the LLM

`radiant` needs an LLM provider and API key to run specs.

```bash
# Option 1: environment variable
export OPENROUTER_API_KEY="sk-or-..."
export RADIANT_MODEL="deepseek/deepseek-chat"

# Option 2: configure inline
radiant config --provider=openrouter --model=deepseek/deepseek-chat --api-key=sk-or-...

# Option 3: Anthropic direct
export ANTHROPIC_API_KEY="sk-ant-..."
radiant config --provider=anthropic --model=claude-sonnet-4-5
```

Supported providers: `openrouter` (default), `openai`, `anthropic`,
any OpenAI-compatible gateway.

## Verify the install

```bash
radiant doctor
```

Should print all ✓ with no ✗.

## Updating

```bash
# reinstall the latest
go install github.com/quant-risk/radiant-harness/cmd/radiant@latest

# refresh bundled skills in your project
cd your-project
radiant update

# regenerate native agent views for the new skills
radiant views --agent=claude,cursor --force
```

## Uninstalling

```bash
# the binary
rm $(which radiant)

# in a project (removes everything radiant wrote)
rm -rf .radiant-harness/
# AGENTS.md, docs/architecture/adr/, docs/product/, specs/ — only
# if you want them gone too.
```

## Troubleshooting

### `radiant: command not found`

`$GOPATH/bin` is not on your `$PATH`. Add it:

```bash
# ~/.zshrc or ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

### `dyld: missing LC_UUID load command` (macOS arm64)

Known issue with Go 1.22.x on Apple Silicon. The Makefile
defaults `CGO_ENABLED=0`; if you built without it, rebuild with:

```bash
CGO_ENABLED=0 go build -o bin/radiant ./cmd/radiant/
```

### API key rejected

Check:

```bash
echo $OPENROUTER_API_KEY  # or $ANTHROPIC_API_KEY
radiant config --show     # print current config
```

### Cross-compile fails on a non-Linux host

The Makefile uses `GOOS` / `GOARCH` env vars. On Windows, use
`make release` from a Git Bash or WSL shell.

## Next

- [README](../README.md) — overview
- [Methodology merge report](../docs/METHODOLOGY-MERGE-FINAL.md) — context
- [Skill schema](../docs/SKILL-SCHEMA.md) — open spec for new skills