# Skill: cli

> CLI tool design: POSIX conventions, argument parsing, subcommand
> shape, output discipline, exit codes, distribution. A CLI that
> logs to stdout is a CLI that breaks `pipe | grep`.

## Decision tree

```
Project starts (or pivots to CLI tool)
        │
        ▼
[Step 1] UX spec              ── docs/cli/ux-spec.md
        │                          (subcommands, flags, outputs)
        ▼
[Step 2] Output discipline    ── stdout vs stderr, exit codes
        │
        ▼
[Step 3] Argument parsing     ── Cobra / clap / Click / argparse
        │
        ▼
[Step 4] Configuration        ── flags > env > config file
        │
        ▼
[Step 5] Error handling       ── docs/cli/error-codes.md
        │
        ▼
[Step 6] Shell completion     ── bash, zsh, fish
        │
        ▼
[Step 7] Distribution         ── install + uninstall paths
        │
        ▼
[Step 8] Man page + docs      ── discoverable + scriptable
```

## Workflow

### Step 1: UX spec

`docs/cli/ux-spec.md` — the contract with users. Lock it BEFORE
implementation.

```yaml
# Example shape (YAML for readability; can be TOML, JSON, etc.)

name: radiant
binary: radiant
description: Spec-driven development harness

subcommands:
  - name: init
    description: Initialize a radiant project in the current directory
    args:
      - name: path
        type: string
        required: false
        default: "."
    flags:
      - name: agent
        short: a
        type: list
        default: ["claude"]
        description: Which agent(s) to scaffold for
      - name: force
        short: f
        type: bool
        default: false
        description: Overwrite existing files

global_flags:
  - name: config
    short: c
    type: string
    description: Path to config file
  - name: verbose
    short: v
    type: bool
    description: Increase log verbosity (use up to -vvv)
  - name: quiet
    short: q
    type: bool
    description: Suppress non-error output
  - name: no-color
    type: bool
    description: Disable ANSI color codes

exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Generic error (catch-all for unexpected)
  - code: 2
    meaning: Usage error (invalid args, missing required)
  - code: 3
    meaning: Configuration error (bad config file)
  - code: 4
    meaning: Network error (couldn't reach upstream)
  - code: 64
    meaning: EX_USAGE (BSD sysexits)
```

Once locked, every change is a breaking change for users. Plan
carefully.

### Step 2: Output discipline

**stdout**: primary output only. The thing the user pipes, greps,
or feeds to another tool.

```
$ mytool list-users
alice
bob
carol
```

**stderr**: logs, progress, prompts, warnings, errors.

```
$ mytool sync --verbose
  Connecting to api.example.com...    # stderr
  ✓ Fetched 1234 users                 # stderr
  ✓ Wrote to ./users.json              # stderr
$                                   # stdout is empty
```

Tests for output discipline:
- `mytool list-users > out.txt` — `out.txt` contains ONLY the
  primary output (one user per line, no log noise)
- `mytool sync 2> logs.txt` — `logs.txt` contains the progress
  messages; stdout may be empty or contain only the primary result
- `--quiet` — stderr suppresses progress; only errors remain
- `--json` — stdout is parseable JSON; stderr is logging

Color and TTY:
- ANSI colors only when stderr is a TTY AND `--no-color` not set
- Detect with `isatty(stderr)` (Go: `os.Stderr.Stat()` ModeCharDevice)
- Always provide `--no-color` for users to opt out

Progress bars:
- Only when stderr is a TTY (CI / pipes don't want them)
- Don't update at 60Hz — that's a progress bar's job, but the
  terminal's also a TTY; rate-limit updates to ~10Hz

### Step 3: Argument parsing

Choose a library that matches the language and supports
subcommands + flags + completion:

| Language | Library | Strengths |
|----------|---------|-----------|
| Go | spf13/cobra | Subcommand tree; widely adopted; auto-generates docs |
| Go | urfave/cli | Simpler API; less ceremony than Cobra |
| Rust | clap | Type-safe derive; great errors; completion built-in |
| Python | Click | Decorator-based; subcommands; batteries-included |
| Python | Typer | Type-hint driven; auto help; modern |
| TypeScript | commander | Mature; subcommands; common in Node CLIs |
| TypeScript | yargs | Fluent API; auto-completion for bash/zsh |

Pick one. Don't mix. A tool with two parsers has two APIs.

Subcommand shape:
- **Flat**: `mytool --flag <action>` — only good for tools with
  one job (`grep`, `curl`, `jq`)
- **Hierarchical**: `mytool <noun> <verb>` — good for tools with
  multiple resources (`git`, `docker`, `kubectl`, `gh`, `aws`)

For multi-resource tools, prefer hierarchical. Names should be
stable; renaming a subcommand is a breaking change.

### Step 4: Configuration

Precedence (highest to lowest):
1. Command-line flags (explicit user intent)
2. Environment variables (CI, container-friendly)
3. Project-local config (`.mytoolrc`, `.mytool/config.toml`)
4. User config (`$XDG_CONFIG_HOME/mytool/config.toml`)
5. System config (`/etc/mytool/config.toml`)
6. Built-in defaults

Document this precedence explicitly. Users debugging "why
doesn't my flag work?" need to know where to look.

XDG Base Directory:
- `$XDG_CONFIG_HOME` (default `~/.config`) — config files
- `$XDG_DATA_HOME` (default `~/.local/share`) — tool data
- `$XDG_CACHE_HOME` (default `~/.cache`) — caches
- `$XDG_STATE_HOME` (default `~/.local/state`) — logs, history

Don't write to `$HOME` directly. That's tech debt.

### Step 5: Error handling

`docs/cli/error-codes.md`:

| Code | Name | When | User action |
|------|------|------|-------------|
| 0 | OK | Success | None |
| 1 | Generic | Unexpected error | Check stderr; report bug |
| 2 | Usage | Invalid args / missing required | Run `mytool <cmd> --help` |
| 3 | Config | Config file invalid | Check config syntax + version |
| 4 | Network | Can't reach upstream | Check network / proxy / API key |
| 5 | Auth | Auth failed / token expired | Re-authenticate |
| 64 | EX_USAGE | BSD sysexits.h usage | Run with `--help` |
| 78 | EX_CONFIG | BSD sysexits.h config | Fix config file |

Error messages should be:
- **Actionable**: tell the user what to do ("Run `mytool login`"
  not "Auth failed")
- **Specific**: name the file / field / endpoint that failed
- **Scoped**: error stack traces go to `--verbose` or `--debug`,
  not the default output
- **Exit-coded**: the exit code reflects the category

### Step 6: Shell completion

Every modern CLI supports completion for at least one shell:

```bash
# bash
mytool completion bash > /etc/bash_completion.d/mytool

# zsh
mytool completion zsh > "${fpath[1]}/_mytool"

# fish
mytool completion fish > ~/.config/fish/completions/mytool.fish

# PowerShell
mytool completion powershell > mytool.ps1
```

Static completion: subcommand names + flag names
Dynamic completion: enum values, branch names, resource IDs
(e.g. `mytool checkout <TAB>` → branch list)

If the tool is in your PATH, ship completion generation. Users
discover features via tab completion; missing completion = hidden
features.

### Step 7: Distribution

| Channel | Language(s) | Effort | Reach |
|---------|------------|--------|-------|
| `go install` | Go | Trivial | Go devs |
| Homebrew | Go/Rust (formula) | Low | macOS / Linux devs |
| `apt` repository | Any | Medium | Debian / Ubuntu |
| `scoop` | Any | Low | Windows devs |
| Binary download (GitHub Releases) | Any | Low | Power users |
| `npm install -g` | TypeScript | Trivial | JS devs |
| `pip install` / `uv tool install` | Python | Trivial | Python devs |
| `cargo install` | Rust | Trivial | Rust devs |
| `brew tap` | Any | Medium | First-party control |

Always include:
- `make install` / `make uninstall` for source builds
- Checksum file (`SHA256SUMS`) for binary downloads
- GPG / cosign signature for the binary
- A version command (`mytool --version`) that includes the git SHA
  and build date

### Step 8: Man page + docs

Every command should have:
- A `--help` output that fits in one screen
- A man page (generated from `--help` or hand-written for detail)
- An online docs page with examples
- A README that shows the 5 most common invocations

`mytool --help` should look like:

```
mytool — do something useful

Usage:
  mytool [command] [flags]

Commands:
  init       Initialize a project
  build      Build the project
  deploy     Deploy the project
  completion Generate shell completion
  help       Help about any command

Flags:
  -c, --config string   Path to config file
  -h, --help            Help for mytool
  -v, --verbose         Verbose output
      --version         Version information

Run 'mytool <command> --help' for command-specific help.
```

## CLI-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Logs to stdout | Breaks `pipe | grep` | All logs to stderr; primary output to stdout only |
| Exit code 0 on error | Breaks CI / scripts | Always non-zero on error; document codes |
| Interactive prompt in CI | Hang / timeout | Default to non-interactive; detect TTY; `--yes` to skip |
| Secret in argv | `ps aux` shows it; shell history; logs | stdin or env vars for secrets |
| Config in $HOME | Hidden state; cleanup nightmare | XDG dirs; document the paths |
| Inconsistent flag names | Users can't remember | Pick a style; stick to it; lint the help output |
| No completion | Hidden features | Ship at least bash + zsh completion |
| No uninstall | Tech debt | Document + script it; remove state dirs |
| Manually-rolled parser | Bugs, security | Use a maintained library (cobra, clap, click) |

## Examples

### Example 1: developer tool (Go + Cobra + Homebrew)

```
Language:    Go 1.22
Library:     spf13/cobra
Distribution: Homebrew + go install + binary download
Platforms:  linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
            windows/amd64, windows/arm64

Subcommands:
  init          Initialize a project
  build         Build the project
  deploy        Deploy the project
  logs          Show logs
  completion    Generate shell completion

Flags:
  -c, --config    Config file path
  -v, --verbose   Verbosity (repeatable)
      --no-color  Disable ANSI colors
      --json      Output as JSON (where applicable)

Config: $XDG_CONFIG_HOME/mytool/config.toml
Exit codes: 0, 1, 2, 3, 4, 5
Completion: bash, zsh, fish, PowerShell

Install: brew install mytool
          go install github.com/.../mytool@latest
          curl -L ... | tar xz -C /usr/local/bin
```

### Example 2: cloud resource manager (Python + Click + pip)

```
Language:    Python 3.12
Library:     Click
Distribution: PyPI (pip install / uv tool install)
Platforms:  wherever Python runs (Linux, macOS, Windows)

Subcommands (hierarchical):
  cloud resource list     List resources
  cloud resource create   Create a resource
  cloud resource delete   Delete a resource
  cloud iam grant         Grant IAM permissions
  cloud iam revoke        Revoke IAM permissions
  cloud logs tail         Tail logs from a resource

Flags:
  --region        AWS / GCP region
  --profile       Credentials profile
  --output        text | json | yaml (default text)
  --no-paginate   Disable automatic pagination

Config: $XDG_CONFIG_HOME/cloud/config.toml; ~/.aws/credentials
Exit codes: 0, 1, 2, 3, 4, 5, 64, 78

Install: pip install cloud-cli
          uv tool install cloud-cli
```

### Example 3: system utility (Rust + clap + apt + brew)

```
Language:    Rust 1.79
Library:     clap (derive)
Distribution: Homebrew + apt + cargo install
Platforms:  linux/amd64, linux/arm64, darwin/amd64, darwin/arm64

Subcommands: flat
  disk-analyzer --path <PATH> [--depth N]
  process-tree --pid <PID>
  net-watch --interface <IFACE>

Flags:
  --config     Path to config
  --log-level  trace | debug | info | warn | error
  --no-color

Config: /etc/disk-analyzer.toml (system), $XDG_CONFIG_HOME/...
Exit codes: 0, 1, 2, 3

Install: brew install disk-analyzer
          apt install disk-analyzer
          cargo install disk-analyzer
```

## Anti-patterns

### ❌ Logging to stdout

The single most common CLI bug. stdout is for primary output.
Logs to stdout break `pipe | grep` and `> out.txt` workflows.
**All logs to stderr. Always.**

### ❌ Exit code 0 on error

Scripts check exit codes, not stdout. Every error path returns
non-zero. Document the codes so users can branch on them.

### ❌ Interactive prompts by default

CI / cron / scripts don't have a TTY. Default to non-interactive.
Detect TTY for interactive mode; provide `--yes` / `--no-input`
flags for explicit override.

### ❌ Secret leakage via argv

Tokens in argv show up in `ps`, shell history, log aggregators,
Kubernetes pod specs. Read secrets from env vars or stdin (e.g.
`--token-file <PATH>` or `--token-stdin`).

### ❌ Hidden state in $HOME

Config / data dumped in `$HOME/.mytool/` is tech debt. Use XDG
Base Directory spec; document the paths; ship an uninstaller
that cleans up.

### ❌ Inconsistent flag names

`--verbose` here, `--verbosity` there, `-v` somewhere else.
Pick a style (e.g. kebab-case, with short flags where possible)
and apply it to ALL subcommands. Lint the help output in CI.

### ❌ No uninstall path

If install is documented, uninstall must be too. Includes config
dirs, caches, completions. `make uninstall` or platform-equivalent.

### ❌ Missing completion

Users discover features via tab completion. If your tool doesn't
have bash/zsh completion, half its features are invisible.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Tool hangs in CI | Add `--yes` / `--non-interactive`; detect TTY; fail fast |
| Secrets leaked via argv | Switch to env vars / stdin / `--token-file`; rotate; scan logs |
| Logs broke a user's pipe | Move logs to stderr; bump version; document the change |
| Config in wrong dir | Migrate; document the new path; keep a one-shot migrator |
| Flag name clash with another tool | Rename; deprecate old name for one release; remove |
| Completion broken | Regenerate completions on every release; test in CI |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial CLI scoping (uses cli inputs) |
| `/roadmap` | Track deprecations, version bumps |
| `/setup-ci` | Build the cross-compile + release pipeline for the binary |
| `/security` | Secret handling; arg validation; sandboxing |
| `/diagramar` | Diagram the subcommand tree for the UX spec |