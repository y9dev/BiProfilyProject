// i18n guard: no Cyrillic/Kazakh literals outside src/i18n, ru/kk key parity, and t('literal.key') resolvable in ru.
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const src = join(root, "src");
const problems = [];

const walk = (dir) =>
  readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? walk(join(dir, e.name)) : [join(dir, e.name)],
  );
const rel = (p) => relative(root, p).split(sep).join("/");

// --- dictionaries
const readJson = (p) => JSON.parse(readFileSync(p, "utf8"));
const merge = (t, s) => {
  for (const [k, v] of Object.entries(s)) {
    if (v && typeof v === "object" && t[k] && typeof t[k] === "object") merge(t[k], v);
    else t[k] = v;
  }
  return t;
};
const load = (lang) => {
  const out = merge({}, readJson(join(src, "i18n", `${lang}.json`)));
  const nsDir = join(src, "i18n", "ns");
  if (existsSync(nsDir))
    for (const f of readdirSync(nsDir).sort()) if (f.endsWith(`.${lang}.json`)) merge(out, readJson(join(nsDir, f)));
  return out;
};
const flat = (o, p = "", acc = new Map()) => {
  for (const [k, v] of Object.entries(o)) {
    const key = p ? `${p}.${k}` : k;
    if (v && typeof v === "object") flat(v, key, acc);
    else acc.set(key, v);
  }
  return acc;
};
const ru = flat(load("ru"));
const kk = flat(load("kk"));
for (const k of ru.keys()) if (!kk.has(k)) problems.push(`missing in kk: ${k}`);
for (const k of kk.keys()) if (!ru.has(k)) problems.push(`missing in ru: ${k}`);

// --- source scan
const CYR = /[Ѐ-ӿ]/;
// Blank out comments (keeping newlines) while respecting string/template literals.
const stripComments = (code) => {
  let out = "";
  let i = 0;
  let q = null;
  while (i < code.length) {
    const c = code[i];
    const n = code[i + 1];
    if (q) {
      out += c;
      if (c === "\\") {
        out += n ?? "";
        i += 2;
        continue;
      }
      if (c === q) q = null;
      i++;
    } else if (c === "/" && n === "/") {
      while (i < code.length && code[i] !== "\n") i++;
    } else if (c === "/" && n === "*") {
      const end = code.indexOf("*/", i + 2);
      const stop = end < 0 ? code.length : end + 2;
      out += code.slice(i, stop).replace(/[^\n]/g, " ");
      i = stop;
    } else {
      if (c === '"' || c === "'" || c === "`") q = c;
      out += c;
      i++;
    }
  }
  return out;
};

const files = walk(src).filter((f) => {
  const r = rel(f);
  if (!/\.(jsx|js|ts|tsx)$/.test(r)) return false;
  if (r.startsWith("src/i18n/")) return false;
  if (/\.test\.[a-z]+$/.test(r)) return false;
  return true;
});

const T_CALL = /(?<![\w$.])t\(\s*(["'])([A-Za-z0-9_.-]+)\1/g;
for (const f of files) {
  const code = stripComments(readFileSync(f, "utf8"));
  code.split("\n").forEach((line, idx) => {
    if (CYR.test(line)) problems.push(`${rel(f)}:${idx + 1}: Cyrillic literal: ${line.trim().slice(0, 100)}`);
  });
  for (const m of code.matchAll(T_CALL)) {
    if (!ru.has(m[2])) problems.push(`${rel(f)}: t("${m[2]}") has no key in ru`);
  }
}

if (problems.length) {
  console.error(`check-i18n: ${problems.length} problem(s)`);
  for (const p of problems) console.error(" - " + p);
  process.exit(1);
}
console.log(`check-i18n: OK (${ru.size} keys, ${files.length} files scanned)`);
