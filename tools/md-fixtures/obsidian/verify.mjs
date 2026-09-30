// Compares the fixtures with what a real Obsidian extracts (metadataCache), over the Chrome DevTools Protocol.
//
//   node verify.mjs prepare <workdir>        build a scratch vault and an isolated user-data dir
//   node verify.mjs check <workdir> [port]   read Obsidian's results and compare (default port 9333)
//
// Obsidian-verified cases must match; nerve-defined cases only report how they differ.
import { readFileSync, readdirSync, writeFileSync, mkdirSync, rmSync, copyFileSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const casesDir = join(dirname(fileURLToPath(import.meta.url)), '..', 'cases');
const [cmd, workArg, portArg] = process.argv.slice(2);
if (!['prepare', 'check'].includes(cmd) || !workArg) {
  console.error('usage: node verify.mjs prepare|check <workdir> [port]');
  process.exit(2);
}
const work = resolve(workArg);
const vault = join(work, 'vault');
const caseNames = readdirSync(casesDir).filter((f) => f.endsWith('.md')).sort();

if (cmd === 'prepare') {
  rmSync(work, { recursive: true, force: true });
  mkdirSync(vault, { recursive: true });
  mkdirSync(join(work, 'userdata'));
  for (const f of caseNames) copyFileSync(join(casesDir, f), join(vault, f));
  const vaults = { nwikifixtures0001: { path: vault, ts: Date.now(), open: true } };
  writeFileSync(join(work, 'userdata', 'obsidian.json'), JSON.stringify({ vaults, updateDisabled: true }));
  const args = `--user-data-dir="${join(work, 'userdata')}" --remote-debugging-port=9333`;
  console.log(`Start an isolated Obsidian (it does not touch your own vaults), then run "check":`);
  console.log(`  macOS: open -n -g -a Obsidian --args ${args}`);
  console.log(`  Linux: obsidian ${args}`);
  process.exit(0);
}

// ---- read Obsidian's results ----
async function evaluate(port, expression) {
  const pages = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
  const ws = new WebSocket(pages.find((p) => p.type === 'page').webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener('open', r, { once: true }));
  const reply = new Promise((r) => ws.addEventListener('message', (e) => r(JSON.parse(e.data)), { once: true }));
  ws.send(JSON.stringify({ id: 1, method: 'Runtime.evaluate', params: { expression, awaitPromise: true, returnByValue: true } }));
  const res = (await reply).result;
  ws.close();
  if (res.exceptionDetails) throw new Error(JSON.stringify(res.exceptionDetails));
  return res.result.value;
}

const DUMP = `(async () => {
  const want = ${caseNames.length};
  for (let i = 0; i < 100; i++) {
    const files = app.vault.getMarkdownFiles();
    if (files.length >= want && files.every((f) => app.metadataCache.getFileCache(f))) break;
    await new Promise((r) => setTimeout(r, 100));
  }
  let version; // the app version; the user agent only carries the installer's
  try { version = require('electron').ipcRenderer.sendSync('version'); } catch { version = navigator.userAgent.match(/obsidian\\/([\\d.]+)/)?.[1]; }
  const out = { version, files: {} };
  for (const f of app.vault.getMarkdownFiles()) {
    const c = app.metadataCache.getFileCache(f) || {};
    const text = await app.vault.read(f);
    const enc = new TextEncoder();
    const bytes = (o) => enc.encode(text.slice(0, o)).length;
    const ref = (l) => ({ link: l.link, original: l.original, displayText: l.displayText, span: l.position && [bytes(l.position.start.offset), bytes(l.position.end.offset)] });
    out.files[f.path] = {
      refs: [...(c.links || []), ...(c.embeds || [])].map(ref),
      tags: (c.tags || []).map((t) => t.tag.slice(1)),
      frontmatter: c.frontmatter ?? null,
      firstSection: c.sections?.[0]?.type ?? null,
      propertyRefs: (c.frontmatterLinks || []).map((l) => ({ ...ref(l), key: l.key })),
    };
  }
  return JSON.stringify(out);
})()`;

// ---- normalize Obsidian's results into the fixture's terms ----
// Obsidian reports a link's whole source span (not the target's) and NFC-normalizes link targets,
// so links are matched by kind/target/anchor/display and by the fixture range lying inside that span.
function normalize(r, bomBytes) {
  const o = r.original;
  const kind = o.startsWith('![[') ? 'embed' : o.startsWith('[[') ? 'wikilink' : o.startsWith('![') ? 'image' : 'link';
  const hash = r.link.indexOf('#');
  const target = hash < 0 ? r.link : r.link.slice(0, hash);
  const anchor = hash < 0 ? null : r.link.slice(hash + 1) || null;
  const wiki = kind === 'wikilink' || kind === 'embed';
  const display = wiki && o.includes('|') ? r.displayText : null;
  return { kind, target, anchor, display, key: r.key ?? null, span: r.span && [r.span[0] + bomBytes, r.span[1] + bomBytes] };
}

const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
const nfc = (s) => (s === null ? s : s.normalize('NFC'));
const brief = (l) => `${l.kind}:${nfc(l.target)}${l.anchor ? '#' + nfc(l.anchor) : ''}${l.display !== null ? '|' + l.display : ''}${l.key ? ' @' + l.key : ''}`;

function compare(exp, got, bomBytes) {
  const diffs = [];
  const unmatched = got.refs.map((r) => normalize(r, bomBytes));
  for (const l of exp.links.filter((x) => x.key === null)) {
    const i = unmatched.findIndex((r) => brief(r) === brief(l) && r.span[0] <= l.range[0] && l.range[1] <= r.span[1]);
    if (i < 0) diffs.push(`${brief(l)} at ${l.range}: Obsidian has no such link there`);
    else unmatched.splice(i, 1);
  }
  for (const r of unmatched) diffs.push(`${brief(r)} at ${r.span}: only in Obsidian`);
  const expProps = exp.links.filter((l) => l.key !== null).map(brief).sort();
  const gotProps = got.propertyRefs.map((r) => brief(normalize(r, 0))).sort();
  if (!same(expProps, gotProps)) diffs.push(`property links: fixture [${expProps}] vs Obsidian [${gotProps}]`);
  if (!same(exp.tags, got.tags)) diffs.push(`tags: fixture ${JSON.stringify(exp.tags)} vs Obsidian ${JSON.stringify(got.tags)}`);
  const fm = exp.frontmatter;
  if ((fm !== null) !== (got.firstSection === 'yaml')) diffs.push(`frontmatter presence: fixture ${fm !== null}, Obsidian ${got.firstSection === 'yaml'}`);
  const gotProps2 = got.frontmatter;
  if (fm?.valid === true && !deepEqual(fm.properties, gotProps2)) diffs.push(`properties: fixture ${JSON.stringify(fm.properties)} vs Obsidian ${JSON.stringify(gotProps2)}`);
  if (fm?.valid === false && gotProps2 !== null) diffs.push(`properties: fixture invalid, Obsidian ${JSON.stringify(gotProps2)}`);
  return diffs;
}

function deepEqual(a, b) {
  if (typeof a !== 'object' || a === null || typeof b !== 'object' || b === null) return a === b;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  const ka = Object.keys(a), kb = Object.keys(b);
  return ka.length === kb.length && ka.every((k) => deepEqual(a[k], b[k]));
}

const result = JSON.parse(await evaluate(Number(portArg ?? 9333), DUMP));
console.log(`Obsidian ${result.version}, ${caseNames.length} cases`);
let failed = 0;
for (const f of caseNames) {
  const exp = JSON.parse(readFileSync(join(casesDir, f.replace(/\.md$/, '.json')), 'utf8'));
  const got = result.files[f];
  const src = readFileSync(join(casesDir, f));
  const bom = src[0] === 0xef && src[1] === 0xbb && src[2] === 0xbf ? 3 : 0;
  const diffs = got ? compare(exp, got, bom) : ['missing in Obsidian'];
  if (exp.source === 'obsidian-verified' && diffs.length) {
    failed++;
    console.log(`FAIL ${f}\n  ${diffs.join('\n  ')}`);
  } else if (exp.source === 'nerve-defined') {
    console.log(diffs.length ? `diff ${f} (nerve-defined)\n  ${diffs.join('\n  ')}` : `same ${f} (nerve-defined, but Obsidian agrees: reconsider the source)`);
  }
}
console.log(failed ? `${failed} obsidian-verified case(s) differ` : 'all obsidian-verified cases match');
process.exit(failed ? 1 : 0);
