#!/usr/bin/env node
// Mermaid validator — checks block syntax.
// Usage: npx tsx scripts/validate-mermaid.ts [dir]
import { validateMermaid } from "@radiant/harness";
const result = validateMermaid(process.argv[2] || ".");
if (result.warnings.length) {
  console.log(`\n⚠ Mermaid warnings (${result.warnings.length}):`);
  for (const w of result.warnings) console.log(`  • ${w}`);
}
if (!result.ok) {
  console.error(`\n✗ Mermaid validation: ${result.errors.length} error(s)\n`);
  for (const e of result.errors) console.error(`  • ${e}`);
  process.exit(1);
}
console.log(`✓ Mermaid validation: blocks OK.`);
