#!/usr/bin/env node
// Catalog key-parity check: every locale in each app must define the SAME key set,
// so a missing/extra translation key is caught before it ships as a broken label.
// Dependency-free (no test runner). Run: node scripts/check-i18n-parity.mjs
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const APPS = ["frontend", "career-portal"];
const LOCALES = ["th", "en"];

// Flatten nested keys to dotted paths.
function keys(obj, prefix = "") {
  const out = [];
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === "object" && !Array.isArray(v)) out.push(...keys(v, path));
    else out.push(path);
  }
  return out.sort();
}

let failed = false;
for (const app of APPS) {
  const sets = {};
  for (const loc of LOCALES) {
    const file = join(root, app, "messages", `${loc}.json`);
    sets[loc] = new Set(keys(JSON.parse(readFileSync(file, "utf8"))));
  }
  const [a, b] = LOCALES;
  const missingInB = [...sets[a]].filter((k) => !sets[b].has(k));
  const missingInA = [...sets[b]].filter((k) => !sets[a].has(k));
  if (missingInB.length || missingInA.length) {
    failed = true;
    console.error(`✗ ${app}: catalog key mismatch`);
    if (missingInB.length) console.error(`  missing in ${b}.json: ${missingInB.join(", ")}`);
    if (missingInA.length) console.error(`  missing in ${a}.json: ${missingInA.join(", ")}`);
  } else {
    console.log(`✓ ${app}: ${sets[a].size} keys, ${a}/${b} in parity`);
  }
}
process.exit(failed ? 1 : 0);
