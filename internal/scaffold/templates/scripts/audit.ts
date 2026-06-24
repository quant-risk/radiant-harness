#!/usr/bin/env node
// Pipeline audit — validates structure, frontmatter, links, specs.
// Usage: npx tsx scripts/audit.ts [dir]
import { auditPipeline } from "@radiant/harness";
const result = auditPipeline(process.argv[2] || ".");
if (!result.ok) {
  console.error(`\n✗ Pipeline audit: ${result.errors.length} problem(s)\n`);
  for (const e of result.errors) console.error(`  • ${e}`);
  process.exit(1);
}
console.log(`✓ Pipeline audit: ${result.details?.filesScanned} docs OK.`);
