#!/usr/bin/env node
// SessionStart hook — injects base SDD context (alwaysApply: true docs) at session start.
// stdout from this script is added to Claude Code's context.
// Runs from project root; reads only what exists.

import { readFileSync, existsSync } from "node:fs";

// "alwaysApply: true" docs — base context for every session.
const BASE = [
  "docs/STATE.md",
  "docs/product/vision.md",
  "docs/product/roadmap.md",
];

// CORRECTION: Also load the active spec if STATE.md points to one.
function findActiveSpec() {
  if (!existsSync("docs/STATE.md")) return null;
  const state = readFileSync("docs/STATE.md", "utf8");
  // Look for specs/NNNN-<name>/ pattern in the "in progress" section
  const match = state.match(/specs\/\d{4}-[a-z0-9-]+\//);
  if (!match) return null;
  const specPath = `${match[0]}spec.md`;
  return existsSync(specPath) ? specPath : null;
}

let out = "# Base SDD Context (loaded at SessionStart)\n";
out += "> These are the `alwaysApply: true` docs. Others are on demand — pull by `description`.\n";

let any = false;
for (const f of BASE) {
  if (existsSync(f)) {
    out += `\n===== ${f} =====\n${readFileSync(f, "utf8").trim()}\n`;
    any = true;
  }
}

// CORRECTION: Load the active spec, not just a note about it.
const activeSpec = findActiveSpec();
if (activeSpec) {
  out += `\n===== ${activeSpec} (active spec) =====\n${readFileSync(activeSpec, "utf8").trim()}\n`;
  any = true;
} else {
  out += "\n> No active spec found in STATE.md. Run /nova-feature to start one.\n";
}

if (any) process.stdout.write(out);
