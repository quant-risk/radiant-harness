#!/usr/bin/env node
// Spec fidelity eval — AC→task→test traceability.
// Usage: npx tsx scripts/eval-spec-fidelity.ts [dir]
import { evalSpecFidelity } from "@radiant/harness";
const result = evalSpecFidelity(process.argv[2] || ".");
if (result.warnings.length) {
  for (const w of result.warnings) console.log(`  ⚠ ${w}`);
}
if (!result.ok) {
  console.error(`\n✗ ${result.errors.length} AC without task coverage.\n`);
  for (const e of result.errors) console.error(`  • ${e}`);
  process.exit(1);
}
console.log(`\n✓ Spec→task traceability OK.`);
