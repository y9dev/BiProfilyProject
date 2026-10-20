// Legacy guard: removed files must not return or be imported; no demo/fixture code, raw HTML injection or stub pages in src.
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const src = join(root, "src");
const rel = (p) => relative(root, p).split(sep).join("/");
const walk = (dir) =>
  readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? walk(join(dir, e.name)) : [join(dir, e.name)],
  );

// Paths deleted in Phase 9 (legacy demo app); they must stay gone.
const deleted = [
  "Admin", "Builder", "Catalog", "Demo", "DynamicsChart", "ProfilePage", "ResultWidget", "Team", "TestsList",
].map((n) => `src/${n}.jsx`).concat([
  "src/App.css", "src/assets", "src/pages", "src/components", "src/ui/StubPage.jsx",
]);
// Import specifiers that must not appear (legacy dirs and files).
const BAD_IMPORT = /from\s+["'][^"']*\/(pages|components|assets)\/|from\s+["']\.\/?(Admin|Builder|Catalog|Demo|DynamicsChart|ProfilePage|ResultWidget|Team|TestsList)(\.jsx)?["']|App\.css|StubPage/;
const BAD_CODE = [
  [/\bDemo\b/, "demo reference"],
  [/\bFIXTURE/, "fixture reference"],
  [/dangerouslySetInnerHTML/, "dangerouslySetInnerHTML"],
];

const problems = [];
for (const d of deleted) if (existsSync(join(root, d))) problems.push(`legacy path still exists: ${d}`);

for (const f of walk(src)) {
  const r = rel(f);
  if (!/\.(jsx?|tsx?|css)$/.test(r) || /\.test\.[a-z]+$/.test(r)) continue;
  readFileSync(f, "utf8").split("\n").forEach((line, i) => {
    if (BAD_IMPORT.test(line)) problems.push(`${r}:${i + 1}: imports a deleted legacy path`);
    for (const [re, what] of BAD_CODE) if (re.test(line)) problems.push(`${r}:${i + 1}: ${what}`);
  });
}

if (problems.length) {
  console.error(`check-legacy: ${problems.length} problem(s)`);
  for (const p of problems) console.error(" - " + p);
  process.exit(1);
}
console.log(`check-legacy: OK (${deleted.length} deleted paths absent)`);
