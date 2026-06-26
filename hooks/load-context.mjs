#!/usr/bin/env node
// SessionStart hook v2 — token-aware, lazy loading via CONTEXT.md.
// Sprint 36: loads only the pre-assembled CONTEXT.md (≤2KB), not full skill files.
// Falls back to legacy BASE docs when no CONTEXT.md exists yet.

import { readFileSync, existsSync, statSync } from "node:fs";

const CONTEXT_FILE = ".radiant-harness/CONTEXT.md";
const MAX_BYTES = 2048; // 2KB overhead cap

function findActiveSpec() {
  if (!existsSync("docs/STATE.md")) return null;
  const state = readFileSync("docs/STATE.md", "utf8");
  const match = state.match(/specs\/\d{4}-[a-z0-9-]+\//);
  if (!match) return null;
  const specPath = `${match[0]}spec.md`;
  return existsSync(specPath) ? specPath : null;
}

// ── Fast path: use pre-assembled CONTEXT.md ──────────────────────────────────
if (existsSync(CONTEXT_FILE)) {
  const stat = statSync(CONTEXT_FILE);
  let content = readFileSync(CONTEXT_FILE, "utf8");

  // Trim to MAX_BYTES if needed (hard cap — full compression runs at assemble time)
  if (stat.size > MAX_BYTES) {
    content = content.slice(0, MAX_BYTES) + "\n\n[context trimmed — run `radiant context assemble` to refresh]\n";
  }

  process.stdout.write(content);
  process.exit(0);
}

// ── Fallback: legacy load of base docs ───────────────────────────────────────
const BASE = [
  "docs/STATE.md",
  "docs/product/vision.md",
  "docs/product/roadmap.md",
];

let out = "# Base SDD Context (loaded at SessionStart)\n";
out += "> No .radiant-harness/CONTEXT.md found. Run `radiant context assemble` for token-efficient loading.\n";

let any = false;
for (const f of BASE) {
  if (existsSync(f)) {
    out += `\n===== ${f} =====\n${readFileSync(f, "utf8").trim()}\n`;
    any = true;
  }
}

const activeSpec = findActiveSpec();
if (activeSpec) {
  out += `\n===== ${activeSpec} (active spec) =====\n${readFileSync(activeSpec, "utf8").trim()}\n`;
  any = true;
} else {
  out += "\n> No active spec found in STATE.md. Run /nova-feature to start one.\n";
}

if (any) process.stdout.write(out);
