# radiant-harness — Claude Code Instructions

> **Do not write code, run tests, or modify files directly.**
> All work must go through the `radiant` CLI.

---

## Start here

```bash
radiant doctor    # sanity-check the environment
radiant boot      # read the project manifest (≤500 tokens)
```

---

## Running a task

```bash
# Single goal — autonomous loop
radiant loop start "<goal>" --profile=standard

# Monitor
radiant loop status
radiant loop list

# Resume if interrupted
radiant loop resume <run-id>
```

## Running a multi-agent task

```bash
# Parallel fleet
radiant fleet start "<goal>"
radiant fleet status <run-id>
radiant fleet resume <run-id>
```

## After completion

```bash
radiant loop export <run-id>     # trace as Markdown
radiant loop diff <run-id>       # git diff
```

---

## Rules

- Never edit files before starting a loop — the loop does it.
- Never run `go test` or `go build` directly — the Verify phase does it.
- If a loop fails → `radiant loop resume <run-id>` first.
- Use `radiant doctor` if anything feels wrong.

---

## Full command reference

See [AGENTS.md](AGENTS.md) for the complete command reference, profiles, flags, and config.
