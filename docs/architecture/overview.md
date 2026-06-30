---
name: architecture-overview
description: System architecture across 5 axes + security and operational. Pull when working on architecture, infra, quality, observability, or security.
alwaysApply: false
---

# System Architecture

## 1. Tech stack

- Go module `github.com/quant-risk/radiant-harness/v3`.
- Cobra-style CLI under `cmd/radiant`.
- Local MCP JSON-RPC server for host-agent integration.
- Bash/Python validation and release scripts.
- Markdown specs, docs, skills, and state files as first-class artifacts.

## 2. Base architecture

radiant-harness is a local CLI plus MCP runtime. The main bounded contexts are:

- CLI commands: install, doctor, setup, loop, fleet, MCP, docs, telemetry.
- MCP possession: `radiant_possess`, self-driven fallback, async/offline
  primitives, phase status.
- Loop engine: single-goal Discover -> Plan -> Execute -> Verify -> Persist.
- Fleet engine: decomposition, dispatch, retry, and summary.
- Skills and ontology: bundled domain instructions and compact world model.
- Install/release: cross-agent setup, drop-in installer, release artifacts.

## 3. Infra

The harness is local-first. It installs a single `radiant` binary, writes MCP
configuration for supported host agents, and persists state under
`.radiant-harness/`. Public distribution is through GitHub releases and
`install.sh`.

## 4. Quality

Quality is enforced by Go tests, MCP self-test, install audit, drop-in E2E,
agent matrix checks, and release artifact validation. `scripts/run.sh` is the
project-level verification entrypoint.

## 5. Observability

Runs persist state, traces, and handoff files under `.radiant-harness/`.
Loop/fleet commands expose status/history/export/diff views. MCP tools return
structured JSON-RPC errors for invalid input and operational failures.

## 6. Security

The v3 line avoids direct HTTP LLM calls in the harness runtime. Agent reasoning
is routed through host-provided MCP sampling or self-driven handoff. Gate
execution uses policy allowlists for risky shell commands.

## 7. Operational

Primary operator flow:

1. Install with `install.sh`.
2. Run `radiant setup-mcp --agent=<host>`.
3. Restart the host agent.
4. Use `radiant_possess` through MCP, or self-driven/async primitives when the
   host cannot sample.
5. Validate with `radiant doctor`, `radiant mcp self-test`, and project gates.
