#!/usr/bin/env node
// PreToolUse hook — blocks tool calls when token budget < 10% remaining.
// Sprint 36: enforces budget-first design at the hook layer.
//
// Exit code 2 + JSON to stdout blocks the tool call in Claude Code.
// Exit code 0 allows it through.

import { readFileSync, existsSync } from "node:fs";

const LOOP_STATE = ".radiant-harness/loop.json";
const BUDGET_WARN_FLOOR = 0.10; // block if < 10% remaining

function readLoopState() {
  if (!existsSync(LOOP_STATE)) return null;
  try {
    return JSON.parse(readFileSync(LOOP_STATE, "utf8"));
  } catch {
    return null;
  }
}

const state = readLoopState();
if (!state) process.exit(0); // no active loop — allow all tools

const budget = state.budget;
if (!budget || !budget.max_tokens || budget.max_tokens <= 0) {
  process.exit(0); // unlimited budget — allow
}

const used = budget.used_tokens || 0;
const max = budget.max_tokens;
const remaining = (max - used) / max;

if (remaining < BUDGET_WARN_FLOOR) {
  // Block the tool call — return structured JSON for Claude Code
  const response = {
    decision: "block",
    reason: `Token budget exhausted: ${used}/${max} tokens used (${(remaining * 100).toFixed(1)}% remaining, minimum 10% required). Run \`radiant loop status\` to check state.`,
  };
  process.stdout.write(JSON.stringify(response) + "\n");
  process.exit(2);
}

// Budget OK — allow tool call
process.exit(0);
