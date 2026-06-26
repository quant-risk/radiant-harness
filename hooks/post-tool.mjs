#!/usr/bin/env node
// PostToolUse hook — records every tool call into the active loop trace.
// Sprint 36: enables reasoning trace continuity across tool invocations.
//
// Claude Code passes tool info via stdin as JSON:
// { "tool_name": "...", "tool_input": {...}, "tool_response": {...} }

import { readFileSync, writeFileSync, existsSync, mkdirSync, appendFileSync } from "node:fs";
import { createHash } from "node:crypto";

const LOOP_STATE = ".radiant-harness/loop.json";
const TRACES_DIR = ".radiant-harness/traces";

function readLoopState() {
  if (!existsSync(LOOP_STATE)) return null;
  try {
    return JSON.parse(readFileSync(LOOP_STATE, "utf8"));
  } catch {
    return null;
  }
}

function promptHash(input) {
  return createHash("sha256")
    .update(JSON.stringify(input))
    .digest("hex")
    .slice(0, 8);
}

// Read tool event from stdin
let raw = "";
try {
  raw = readFileSync("/dev/stdin", "utf8");
} catch {
  process.exit(0); // no stdin — nothing to record
}

let event;
try {
  event = JSON.parse(raw);
} catch {
  process.exit(0);
}

const state = readLoopState();
if (!state) process.exit(0); // no active loop — skip silently

const runID = state.run_id;
const phase = state.phase || "unknown";

mkdirSync(TRACES_DIR, { recursive: true });
const tracePath = `${TRACES_DIR}/${runID}.jsonl`;

const entry = {
  ts: new Date().toISOString(),
  run: runID,
  phase,
  action: event.tool_name || "unknown_tool",
  prompt_hash: promptHash(event.tool_input || {}),
  result: "ok",
  evidence: summarizeResponse(event.tool_response),
  meta: { tool: event.tool_name },
};

appendFileSync(tracePath, JSON.stringify(entry) + "\n", "utf8");

function summarizeResponse(response) {
  if (!response) return "";
  const str = typeof response === "string" ? response : JSON.stringify(response);
  return str.slice(0, 120).replace(/\n/g, " ");
}
