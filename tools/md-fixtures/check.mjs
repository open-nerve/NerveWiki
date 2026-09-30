// Checks that every fixture is well-formed and self-consistent:
// each range points at the bytes where its target is written.
// Usage: node tools/md-fixtures/check.mjs
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const SOURCES = new Set(["obsidian-verified", "nerve-defined"]);
const KINDS = new Set(["wikilink", "embed", "link", "image"]);
const problems = [];
const fail = (name, msg) => problems.push(`${name}: ${msg}`);

const isNullOrText = (v) => v === null || (typeof v === "string" && v !== "");

function decode(s) {
  try {
    return decodeURI(s);
  } catch {
    return s;
  }
}

// Byte span of the YAML between the frontmatter delimiters, or null.
function frontmatterSpan(src) {
  let start = src[0] === 0xef && src[1] === 0xbb && src[2] === 0xbf ? 3 : 0;
  const text = src.toString("latin1"); // 1 char per byte keeps offsets in bytes
  const open = text.startsWith("---\n", start) ? 4 : text.startsWith("---\r\n", start) ? 5 : 0;
  if (!open) return null;
  for (let pos = start + open; pos < text.length;) {
    const nl = text.indexOf("\n", pos);
    const end = nl < 0 ? text.length : nl;
    if (text.slice(pos, end).replace(/\r$/, "") === "---") return { from: start + open, to: pos };
    if (nl < 0) break;
    pos = nl + 1;
  }
  return null;
}

function checkLink(name, src, fm, l, i) {
  const at = `links[${i}]`;
  if (!KINDS.has(l.kind)) return fail(name, `${at}.kind ${l.kind}`);
  if (typeof l.target !== "string" || l.target === "") fail(name, `${at}.target must be non-empty`);
  if (!isNullOrText(l.anchor) || !isNullOrText(l.display))
    fail(name, `${at}: anchor/display must be null or non-empty`);
  if (!(l.key === null || (typeof l.key === "string" && l.key !== "")))
    fail(name, `${at}.key must be null or non-empty`);
  const [s, e] = l.range ?? [];
  if (!Number.isInteger(s) || !Number.isInteger(e) || s < 0 || e <= s || e > src.length)
    return fail(name, `${at}.range ${l.range}`);
  const inFrontmatter = fm !== null && s >= fm.from && e <= fm.to;
  if ((l.key !== null) !== inFrontmatter) fail(name, `${at}: key must be set exactly for links inside the frontmatter`);
  const written = src.subarray(s, e).toString("utf8");
  let ok;
  if (l.kind === "link" || l.kind === "image") ok = decode(written) === l.target;
  else ok = written === l.target || (l.key !== null && written.replaceAll("''", "'") === l.target);
  if (!ok) fail(name, `${at}: bytes ${JSON.stringify(written)} do not spell target ${JSON.stringify(l.target)}`);
}

function checkCase(dir, base) {
  const name = `cases/${base}`;
  const jsonPath = join(dir, `${base}.json`);
  if (!existsSync(jsonPath)) return fail(name, "missing .json");
  const src = readFileSync(join(dir, `${base}.md`));
  const exp = JSON.parse(readFileSync(jsonPath, "utf8"));
  const keys = Object.keys(exp).toSorted().join(",");
  const want =
    exp.source === "nerve-defined"
      ? "description,frontmatter,links,note,source,tags"
      : "description,frontmatter,links,source,tags";
  if (keys !== want) fail(name, `fields ${keys}, want ${want}`);
  if (!SOURCES.has(exp.source)) fail(name, `source ${exp.source}`);
  const fm = exp.frontmatter;
  const fmOk =
    fm === null ||
    (fm.valid === false && Object.keys(fm).length === 1) ||
    (fm.valid === true && typeof fm.properties === "object" && fm.properties !== null && !Array.isArray(fm.properties));
  if (!fmOk) fail(name, "frontmatter must be null, {valid:false} or {valid:true, properties:{…}}");
  const span = frontmatterSpan(src);
  if ((fm === null) !== (span === null)) fail(name, "frontmatter presence does not match the source");
  exp.links.forEach((l, i) => checkLink(name, src, span, l, i));
  for (let i = 1; i < exp.links.length; i++) {
    if (exp.links[i].range[0] < exp.links[i - 1].range[1])
      fail(name, `links[${i}] is out of order or overlaps the previous range`);
  }
  if (!Array.isArray(exp.tags) || exp.tags.some((t) => typeof t !== "string" || t === "" || t.startsWith("#")))
    fail(name, "tags must be non-empty strings without #");
}

function checkRename(dir, base) {
  const name = `rename/${base}`;
  for (const ext of [".md", ".out.md"]) if (!existsSync(join(dir, base + ext))) fail(name, `missing ${ext}`);
  const exp = JSON.parse(readFileSync(join(dir, `${base}.json`), "utf8"));
  if (Object.keys(exp).toSorted().join(",") !== "description,from,source,to")
    fail(name, "fields must be description, source, from, to");
  if (!SOURCES.has(exp.source)) fail(name, `source ${exp.source}`);
}

const casesDir = join(root, "cases");
const cases = readdirSync(casesDir)
  .filter((f) => f.endsWith(".md"))
  .map((f) => f.slice(0, -3));
cases.forEach((b) => checkCase(casesDir, b));
const renameDir = join(root, "rename");
const renames = readdirSync(renameDir)
  .filter((f) => f.endsWith(".json"))
  .map((f) => f.slice(0, -5));
renames.forEach((b) => checkRename(renameDir, b));

if (problems.length) {
  console.error(problems.join("\n"));
  process.exit(1);
}
console.log(`ok: ${cases.length} cases, ${renames.length} rename cases`);
