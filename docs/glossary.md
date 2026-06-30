---
name: glossary
description: Ubiquitous language. Pull when naming, modeling domain, or writing specs.
alwaysApply: false
---

# Glossary — Ubiquitous Language

| Term | Definition | Do NOT confuse with | Context |
|------|------------|---------------------|---------|
| Boot | Compact project manifest emitted for orientation. | A full execution loop. | CLI/runtime |
| Drop-in install | Installing radiant-harness from GitHub into another project with minimal setup. | Local development checkout. | Install |
| Fleet | Parallel multi-agent decomposition for independent subtasks. | Loop, which is a single-goal flow. | Orchestration |
| Host agent | The agent runtime using radiant, such as Codex, Claude Code, Cursor, or another MCP-capable tool. | The radiant harness itself. | MCP |
| Loop | Crash-safe Discover -> Plan -> Execute -> Verify -> Persist cycle for one goal. | Fleet. | Orchestration |
| MCP sampling | Host-provided `sampling/createMessage` callback used by the harness to ask the agent for phase work. | Direct HTTP LLM calls. | MCP |
| Ontology | Compact world model and domain vocabulary used to keep tasks, specs, skills, and architecture aligned. | A database schema. | Context |
| Possession | Harness-guided mode where radiant takes over the agent workflow for a bounded task. | Shell command execution alone. | MCP |
| Self-driven mode | Fallback mode for hosts without sampling; radiant writes scaffolded handoff files and the host agent fills them using native tools. | A failed or empty stub. | MCP |
| Skill | Domain or workflow instruction pack loaded by radiant to guide agent behavior. | A plugin binary. | Skills |
| Spec | Task-specific contract containing goal, acceptance criteria, non-goals, and verification gates. | Product roadmap. | Specs |
