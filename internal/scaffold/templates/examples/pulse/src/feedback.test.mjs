// Acceptance tests — Collect feedback. One per AC (spec→test traceability).
import { test } from "node:test";
import assert from "node:assert/strict";
import { submitFeedback } from "./feedback.mjs";

test("AC-1: valid feedback is accepted and returns id", () => {
  const r = submitFeedback({ text: "great product", context: "/home" });
  assert.ok(r.id, "should return an id");
});

test("AC-2: empty feedback is rejected", () => {
  const r = submitFeedback({ text: "   " });
  assert.ok(r.error, "should return error");
});

test("AC-3: oversized feedback is rejected", () => {
  const r = submitFeedback({ text: "x".repeat(1001) });
  assert.ok(r.error, "should return error");
});
