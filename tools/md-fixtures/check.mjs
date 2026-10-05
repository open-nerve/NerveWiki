// Checks that every fixture is well-formed and self-consistent:
// each range points at the bytes where its target is written,
// each task's offset at the character between its brackets,
// each resolution case's links go from and to its pages.
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

// The characters between a task's brackets: Go's \s, or x for a done one.
const OPEN = new Set([0x20, 0x09, 0x0a, 0x0c, 0x0d]);
const DONE = new Set([0x78, 0x58]);

function checkTasks(name, src, tasks) {
  if (!Array.isArray(tasks) || tasks.length === 0) return fail(name, "tasks must be a non-empty array, or absent");
  tasks.forEach((t, i) => {
    const at = `tasks[${i}]`;
    if (Object.keys(t).toSorted().join(",") !== "checked,offset")
      return fail(name, `${at}: fields must be offset, checked`);
    const o = t.offset;
    if (!Number.isInteger(o) || o < 1 || o + 1 >= src.length) return fail(name, `${at}.offset ${o}`);
    if (src[o - 1] !== 0x5b || src[o + 1] !== 0x5d) fail(name, `${at}: offset ${o} is not between brackets`);
    if (!(t.checked ? DONE : OPEN).has(src[o]))
      fail(name, `${at}: byte ${src[o]} is not a ${t.checked ? "done" : "open"} task's`);
    if (i > 0 && o <= tasks[i - 1].offset) fail(name, `${at} is out of order`);
  });
}

function checkCase(dir, base) {
  const name = `cases/${base}`;
  const jsonPath = join(dir, `${base}.json`);
  if (!existsSync(jsonPath)) return fail(name, "missing .json");
  const src = readFileSync(join(dir, `${base}.md`));
  const exp = JSON.parse(readFileSync(jsonPath, "utf8"));
  const keys = Object.keys(exp)
    .filter((k) => k !== "tasks")
    .toSorted()
    .join(",");
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
  if ("tasks" in exp) checkTasks(name, src, exp.tasks);
}

const parentOf = (page) => (page.includes("/") ? page.slice(0, page.lastIndexOf("/")) : null);
const sameKeys = (o, want) => Object.keys(o).every((k) => want.includes(k));

// titleKey approximates the server's title key (NFC, Unicode case folding, NFC): JavaScript
// has no case folding, and upper then lower case folds as it does for the titles of cases
// (ß, ﬃ), so that siblings that would share a key are caught.
const titleKey = (s) => s.normalize("NFC").toUpperCase().toLowerCase().normalize("NFC");

// titleError is why the server refuses s as a title (shared.CheckTitle), or "".
function titleError(s) {
  if (s === "" || s !== s.trim()) return "empty, or with spaces around it";
  if (s !== s.normalize("NFC")) return "not NFC";
  if (Buffer.byteLength(s) > 255) return "longer than 255 bytes";
  if (/[\\/:*?"<>|#^[\]\p{Cc}\p{Zl}\p{Zp}\p{Bidi_Control}]/u.test(s))
    return 'with one of / \\ : * ? " < > | # ^ [ ] or a control, line or paragraph separator, or bidi control';
  if (s.startsWith(".") || s.endsWith(".")) return "starting or ending with a dot";
  if (/^(con|prn|aux|nul|com[1-9¹²³]|lpt[1-9¹²³])$/i.test(s.split(".")[0])) return "a name Windows reserves";
  return "";
}

const titleOf = (page) => page.slice(page.lastIndexOf("/") + 1);
const siblingKey = (page) => `${parentOf(page)}/${titleKey(titleOf(page))}`;

// checkTree checks a case's pages, a tree of titles the server takes (each page's parent listed
// before it, no siblings that would share a title key), and its aliases, of listed pages; it
// returns the pages.
function checkTree(name, list, aliasesOf) {
  const pages = new Set();
  const keys = new Set();
  for (const p of list) {
    const why =
      typeof p === "string"
        ? p
            .split("/")
            .map(titleError)
            .find((e) => e !== "")
        : "not a string";
    const key = typeof p === "string" ? siblingKey(p) : "";
    if (why !== undefined) fail(name, `page ${JSON.stringify(p)}: a segment is ${why}`);
    else if (keys.has(key)) fail(name, `page ${p}: a sibling has its title key`);
    else if (parentOf(p) !== null && !pages.has(parentOf(p)))
      fail(name, `page ${p}: its parent must be listed before it`);
    pages.add(p);
    keys.add(key);
  }
  for (const [p, aliases] of Object.entries(aliasesOf ?? {})) {
    if (!pages.has(p)) fail(name, `aliases of ${p}, which is not a page`);
    if (!Array.isArray(aliases) || aliases.length === 0 || aliases.some((a) => typeof a !== "string" || a === ""))
      fail(name, `aliases of ${p} must be non-empty strings`);
  }
  return pages;
}

// A rename case: its keys those the README lists; its pages a tree as a resolution case's (none
// listed: from and page, at the root); from and page among them; to either a rename (the same
// parent, another title) or a move (the same title, a parent that is a page or the root, out of
// from's subtree), with no sibling there that has its title key; a nerve-defined case says in
// its note what Obsidian does.
function checkRename(dir, base) {
  const name = `rename/${base}`;
  for (const ext of [".md", ".out.md"]) if (!existsSync(join(dir, base + ext))) fail(name, `missing ${ext}`);
  let c;
  try {
    c = JSON.parse(readFileSync(join(dir, `${base}.json`), "utf8"));
  } catch (e) {
    return fail(name, `not JSON: ${e.message}`);
  }
  if (!sameKeys(c, ["description", "source", "pages", "aliases", "page", "from", "to", "note"]))
    fail(name, "fields are description, source, from, to, and pages, aliases, page and note when set");
  if (typeof c.description !== "string" || c.description === "") fail(name, "description must be non-empty");
  if (!SOURCES.has(c.source)) fail(name, `source ${c.source}`);
  if ((c.source === "nerve-defined") !== (typeof c.note === "string" && c.note !== ""))
    fail(name, "note must be set exactly when the case is nerve-defined");
  if (typeof c.from !== "string" || typeof c.to !== "string") return fail(name, "from and to must be paths");
  const page = c.page ?? "src";
  if (c.pages === undefined && c.aliases !== undefined) fail(name, "aliases need pages");
  const pages = checkTree(name, c.pages ?? [c.from, page], c.aliases);
  if (!pages.has(c.from)) return fail(name, `from ${c.from} is not a page`);
  if (!pages.has(page)) fail(name, `page ${page} is not a page`);
  const why = titleError(titleOf(c.to));
  if (why !== "") return fail(name, `to ${JSON.stringify(c.to)}: its title is ${why}`);
  const parent = parentOf(c.to);
  if (parent === parentOf(c.from)) {
    if (titleOf(c.to) === titleOf(c.from)) fail(name, "to is from: nothing is renamed");
  } else if (titleOf(c.to) !== titleOf(c.from)) {
    fail(name, "to changes both the parent and the title: a rename or a move does one");
  } else if (parent !== null && (!pages.has(parent) || parent === c.from || parent.startsWith(`${c.from}/`))) {
    fail(name, `to's parent ${parent} must be a page out of from's subtree`);
  }
  if ([...pages].some((p) => p !== c.from && siblingKey(p) === siblingKey(c.to)))
    fail(name, `to ${c.to}: a sibling there has its title key`);
}

// A resolution case: its keys those the README lists; its pages a tree of titles the server
// takes (each page's parent listed before it, no siblings that would share a title key), its
// aliases of listed pages, each link from a listed page to a listed page or none; a case or a
// link that is nerve-defined says why in the case's note.
function checkResolveCase(dir, base) {
  const name = `resolve/${base}`;
  let c;
  try {
    c = JSON.parse(readFileSync(join(dir, base), "utf8"));
  } catch (e) {
    return fail(name, `not JSON: ${e.message}`);
  }
  if (!sameKeys(c, ["description", "source", "pages", "aliases", "links", "note"]))
    fail(name, "fields are description, source, pages, links, and aliases and note when set");
  if (typeof c.description !== "string" || c.description === "") fail(name, "description must be non-empty");
  if (!SOURCES.has(c.source)) fail(name, `source ${c.source}`);
  if (!Array.isArray(c.pages) || c.pages.length === 0) return fail(name, "pages must be a non-empty array");
  const pages = checkTree(name, c.pages, c.aliases);
  if (!Array.isArray(c.links) || c.links.length === 0) return fail(name, "links must be a non-empty array");
  let nerveDefined = c.source === "nerve-defined";
  c.links.forEach((l, i) => {
    const at = `links[${i}]`;
    if (!sameKeys(l, ["from", "link", "to", "ambiguous", "source"]) || !("to" in l))
      fail(name, `${at}: fields are from, link, to, and ambiguous and source when set`);
    if (!pages.has(l.from)) fail(name, `${at}.from ${l.from} is not a page`);
    if (typeof l.link !== "string" || l.link === "") fail(name, `${at}.link must be non-empty`);
    if (l.to !== null && !pages.has(l.to)) fail(name, `${at}.to ${l.to} is neither null nor a page`);
    if (l.ambiguous !== undefined && (l.ambiguous !== true || l.to === null))
      fail(name, `${at}.ambiguous is true or absent, and true only for a link that resolves`);
    if (l.source !== undefined && !SOURCES.has(l.source)) fail(name, `${at}.source ${l.source}`);
    nerveDefined ||= l.source === "nerve-defined";
  });
  if (nerveDefined !== (typeof c.note === "string" && c.note !== ""))
    fail(name, "note must be set exactly when the case or one of its links is nerve-defined");
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
const resolveDir = join(root, "resolve");
const resolves = readdirSync(resolveDir).filter((f) => f.endsWith(".json"));
resolves.forEach((f) => checkResolveCase(resolveDir, f));

if (problems.length) {
  console.error(problems.join("\n"));
  process.exit(1);
}
console.log(`ok: ${cases.length} cases, ${renames.length} rename cases, ${resolves.length} resolution cases`);
