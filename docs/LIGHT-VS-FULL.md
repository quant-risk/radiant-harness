# Publishing guide: Light vs Full binaries

This document explains the Light/Full split, how to produce each
binary, and how to publish them to separate GitHub repositories
without carrying the other's code.

## What you're publishing

Two physically separate binaries from the **same source tree**:

| Binary | Purpose | Inference source | API key required |
|--------|---------|-------------------|------------------|
| `radiant-light` | Host-agent-driven harness (no LLM HTTP code) | MCP sampling only | **No** |
| `radiant-full`  | Vendor-neutral harness with HTTP LLM clients | HTTP + MCP sampling | Yes |

The Light binary **cannot** talk to Anthropic / OpenAI / OpenRouter /
etc. — those code paths are tag-excluded at compile time. The Full
binary has everything and behaves exactly like prior versions
(v2.47.0 and earlier).

## How to produce each artifact

From the source root:

```bash
# Full — every subcommand, all HTTP LLM providers, requires API key
go build -o ./dist/radiant-full ./cmd/radiant

# Light — only setup-mcp + mcp serve, no API key infrastructure
go build -tags light_only -o ./dist/radiant-light ./cmd/radiant
```

Both cross-compile to:

```
linux/amd64
linux/arm64
darwin/amd64
darwin/arm64
windows/amd64
```

Use the standard `GOOS=... GOARCH=... go build ...` triplet:

```bash
GOOS=linux   GOARCH=amd64 go build -tags light_only -o ./dist/radiant-light-linux-amd64   ./cmd/radiant
GOOS=darwin  GOARCH=arm64 go build -tags light_only -o ./dist/radiant-light-darwin-arm64  ./cmd/radiant
GOOS=windows GOARCH=amd64 go build -tags light_only -o ./dist/radiant-light-windows-amd64.exe ./cmd/radiant
# (same for radiant-full without the -tags flag)
```

## How to publish to two separate repos

This is the user's stated goal: "subir a versão Light num repo e a
versão Full em outro." Three viable approaches:

### Approach A — Same monorepo, two CI release pipelines (recommended)

Keep both source subsets in this monorepo. Set up two GitHub
repositories (`radiant-harness-light` and `radiant-harness-full`)
that pull (or sync) from this monorepo. Each repo's CI runs the
appropriate `go build` invocation.

```
Monorepo:                    quant-risk/radiant-harness
  branches:                  feature/...
  source:                    ./cmd/radiant/...
  contains:                  both Light + Full source files

Mirror 1:                    quant-risk/radiant-harness-light
  CI:                        go build -tags light_only ./cmd/radiant
  release artifacts:         radiant-light-{linux,darwin,windows}-{amd64,arm64}

Mirror 2:                    quant-risk/radiant-harness-full
  CI:                        go build ./cmd/radiant (default)
  release artifacts:         radiant-full-{linux,darwin,windows}-{amd64,arm64}
```

### Approach B — Two copies, each pruned

Before publishing each repo, prune the source to the relevant build
tag:

```bash
# Light repo:
git clone quant-risk/radiant-harness radiant-harness-light
cd radiant-harness-light
# Delete every file tagged //go:build !light_only
# (Or use git-filter-repo with a pathspec that excludes them.)
go build -tags light_only ./cmd/radiant

# Full repo:
git clone quant-risk/radiant-harness radiant-harness-full
cd radiant-harness-full
# Delete cmd_mcp_runtime.go and loop/builder_light.go (Light-only files)
go build ./cmd/radiant
```

### Approach C — Two separate Go modules

If you want absolutely separate go.mod files:

```
radiant-harness-light/
  go.mod    (no internal/llm/anthropic.go, no internal/llm/client.go, no internal/llm/backend_http.go)
  ...

radiant-harness-full/
  go.mod    (everything)
  ...
```

Maintaining parallel go.mod files is the most rigorous separation
but doubles the dependency-management overhead. Approach A or B is
usually fine.

## What each binary does

### Light binary

```bash
$ ./dist/radiant-light --version
2.48.0-light

$ ./dist/radiant-light --help
Available Commands:
  completion  Generate the autocompletion script
  help        Help about any command
  mcp         MCP server commands (Light mode — MCP sampling, no API key)
  setup-mcp   Register radiant as an MCP server in your agent's config
```

Use Light when:
- You're inside an agent (Claude Code / Cursor / Hermes / etc.) that
  provides inference via MCP sampling.
- You don't want to bring your own API key.
- You want the smallest possible binary.
- You want to publish to a vendor-neutral "host-agent-driven"
  channel (no LLM provider baked in).

### Full binary

```bash
$ ./dist/radiant-full --version
2.48.0

$ ./dist/radiant-full --help
Available Commands:
  adr             audit           autodata    bench
  boot            budget          camada-agentica  causal-estimate
  completion      config          context     diagramar
  doctor          drift           eval         evals
  evaluate        fleet           handoff      ...
  (everything)
```

Use Full when:
- You want the harness to call LLM providers directly (Anthropic /
  OpenAI / OpenRouter / Mistral / xAI / Groq / custom).
- You have a vendor relationship and want to bring your own API key.
- You're using the harness in autonomous mode without a host agent.

## Verification

After each build, verify the symbols:

```bash
# Light must have NO HTTP-LLM symbols
nm ./dist/radiant-light | grep -iE 'chatAnthropic|HTTPBackend|NewHTTPBackend'
# (must be empty)

# Full must have all HTTP-LLM symbols
nm ./dist/radiant-full | grep -iE 'HTTPBackend|NewHTTPBackend'
# (must include HTTPBackend and NewHTTPBackend)
```

## CI example (GitHub Actions)

For Light:

```yaml
name: build-light
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Build Light
        run: |
          mkdir -p dist
          for GOOS in linux darwin windows; do
            for GOARCH in amd64 arm64; do
              if [ "$GOOS-$GOARCH" = "windows-arm64" ]; then continue; fi
              ext=""
              [ "$GOOS" = "windows" ] && ext=".exe"
              GOOS=$GOOS GOARCH=$GOARCH go build -tags light_only \
                -o dist/radiant-light-$GOOS-$GOARCH$ext ./cmd/radiant
            done
          done
      - uses: softprops/action-gh-release@v1
        with:
          files: dist/radiant-light-*
          generate_release_notes: true
```

For Full: same workflow without `-tags light_only`.

## Future sprints

- **Sprint 79 — runtime host-agent detection.** The Full binary gains
  `internal/hostdetect/` so an LLM is found at runtime (host sampling
  vs API key) without the user needing to know which mode they're in.
- **Sprint 80 — `radiant host-info` command.** Both binaries print
  detected host agent(s).
