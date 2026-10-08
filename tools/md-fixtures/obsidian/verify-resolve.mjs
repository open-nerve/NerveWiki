// Compares the resolve/ fixtures with how a real Obsidian resolves links, over the Chrome DevTools Protocol.
//
//   node verify-resolve.mjs prepare <workdir>        one scratch vault per case, an isolated user-data dir
//   node verify-resolve.mjs check <workdir> [port]   read Obsidian's resolved links and compare (default port 9333)
//
// A page A is A.md, its children are in A/; an attachment is a file of its name, an image a real one (M7/P3 design
// 4.8), and the vault shows every type of file ("Detect all file extensions"). Each link is a file of its own, holding
// only the link, in the folder of the page it is written in. Obsidian-verified cases must match; nerve-defined ones only
// report how they differ.
import { existsSync, readFileSync, readdirSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const dir = process.env.NWIKI_RESOLVE_DIR ?? join(dirname(fileURLToPath(import.meta.url)), "..", "resolve");
const [cmd, workArg, portArg] = process.argv.slice(2);
if (!["prepare", "check"].includes(cmd) || !workArg) {
  console.error("usage: node verify-resolve.mjs prepare|check <workdir> [port]");
  process.exit(2);
}
const work = resolve(workArg);
const cases = readdirSync(dir)
  .filter((f) => f.endsWith(".json"))
  .toSorted()
  .map((f) => Object.assign(JSON.parse(readFileSync(join(dir, f), "utf8")), { name: f.slice(0, -5) }));
// A 7×5 PNG, for the attachments that are images; any other's bytes are a few of text.
const png = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAcAAAAFCAYAAACJmvbYAAAAEklEQVR42mM4YWPzHxdmGABJADoKTp7oONgaAAAAAElFTkSuQmCC",
  "base64"
);
const fileOf = (name) => (/\.(png|jpe?g|gif|webp|bmp|avif)$/i.test(name) ? png : Buffer.from("not really\n"));
// pathOf is the file a case's page or attachment is in the vault.
const pathOf = (c, to) => (c.assets?.includes(to) ? to : `${to}.md`);
const folderOf = (page) => (page.includes("/") ? page.slice(0, page.lastIndexOf("/")) : "");
const query = (c, i) => join(folderOf(c.links[i].from), `q${String(i).padStart(3, "0")}.md`);

if (cmd === "prepare") {
  // It empties work: only a directory it made before, or none.
  if (existsSync(work) && readdirSync(work).length > 0 && !existsSync(join(work, "userdata", "obsidian.json"))) {
    console.error(`${work} is not empty and was not prepared by this script: choose another directory`);
    process.exit(2);
  }
  rmSync(work, { recursive: true, force: true });
  mkdirSync(join(work, "userdata"), { recursive: true });
  const vaults = {};
  for (const c of cases) {
    const vault = join(work, c.name);
    const write = (p, s) => {
      mkdirSync(dirname(join(vault, p)), { recursive: true });
      writeFileSync(join(vault, p), s);
    };
    for (const p of c.pages) {
      const aliases = c.aliases?.[p];
      write(`${p}.md`, aliases ? `---\naliases: ${JSON.stringify(aliases)}\n---\n` : "");
    }
    for (const a of c.assets ?? []) write(a, fileOf(a));
    write(".obsidian/app.json", JSON.stringify({ showUnsupportedFiles: true }));
    c.links.forEach((l, i) => write(query(c, i), `${l.link}\n`));
    vaults[
      c.name
        .replace(/[^0-9a-z]/g, "")
        .slice(0, 16)
        .padEnd(16, "0")
    ] = { path: vault, ts: Date.now(), open: true };
  }
  writeFileSync(join(work, "userdata", "obsidian.json"), JSON.stringify({ vaults, updateDisabled: true }));
  const args = `--user-data-dir="${join(work, "userdata")}" --remote-debugging-port=9333`;
  console.log(
    `Start an isolated Obsidian (it does not touch your own vaults; it opens one window a case), then run "check":`
  );
  console.log(`  macOS: open -n -g -a Obsidian --args ${args}`);
  console.log(`  Linux: obsidian ${args}`);
  process.exit(0);
}

async function evaluate(target, expression) {
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener("open", r, { once: true }));
  const reply = new Promise((r) => ws.addEventListener("message", (e) => r(JSON.parse(e.data)), { once: true }));
  ws.send(
    JSON.stringify({
      id: 1,
      method: "Runtime.evaluate",
      params: { expression, awaitPromise: true, returnByValue: true },
    })
  );
  const res = (await reply).result;
  ws.close();
  if (res.exceptionDetails) throw new Error(JSON.stringify(res.exceptionDetails));
  return res.result.value;
}

// Each file's links, resolved and not: Obsidian sets both maps of a file together, once it
// resolved its links; until every file has its entry, it has not.
const DUMP = `(async () => {
  const cache = app.metadataCache;
  for (let i = 0; i < 300; i++) {
    const files = app.vault.getMarkdownFiles();
    if (files.length > 0 && files.every((f) => f.path in cache.resolvedLinks && f.path in cache.unresolvedLinks)) break;
    await new Promise((r) => setTimeout(r, 100));
  }
  let version; try { version = require('electron').ipcRenderer.sendSync('version'); } catch { version = navigator.userAgent.match(/obsidian\\/([\\d.]+)/)?.[1]; }
  const out = { version, base: app.vault.adapter.basePath, resolved: {}, unresolved: {} };
  for (const f of app.vault.getMarkdownFiles()) {
    out.resolved[f.path] = Object.keys(cache.resolvedLinks[f.path] ?? {});
    out.unresolved[f.path] = Object.keys(cache.unresolvedLinks[f.path] ?? {});
  }
  return JSON.stringify(out);
})()`;

const port = portArg ?? 9333;
const targets = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).filter((t) => t.type === "page");
const byVault = {};
let version;
for (const dump of await Promise.all(targets.map((t) => evaluate(t, DUMP)))) {
  const out = JSON.parse(dump);
  version = out.version;
  byVault[out.base] = out;
}
console.log(`Obsidian ${version}, ${cases.length} cases, ${Object.keys(byVault).length} vaults open`);
let failed = 0;
for (const c of cases) {
  const vault = byVault[join(work, c.name)];
  if (!vault) {
    console.log(`FAIL ${c.name}: its vault is not open`);
    failed++;
    continue;
  }
  c.links.forEach((l, i) => {
    const got = vault.resolved[query(c, i)];
    const unresolved = vault.unresolved[query(c, i)];
    if (got === undefined || got.length + unresolved.length !== 1) {
      console.log(
        `FAIL ${c.name} ${l.from} ${l.link}: Obsidian took ${got === undefined ? "no" : got.length + unresolved.length} links`
      );
      failed++;
      return;
    }
    const gotPath = got.length === 1 ? got[0] : null;
    const source = l.source ?? c.source;
    if (gotPath === (l.to === null ? null : pathOf(c, l.to))) {
      if (source === "nerve-defined")
        console.log(`same ${c.name} ${l.from} ${l.link}: Obsidian agrees, reconsider nerve-defined`);
      return;
    }
    const line = `${c.name} ${l.from} ${l.link}: Obsidian ${gotPath}, the case ${l.to === null ? null : pathOf(c, l.to)}`;
    if (source === "obsidian-verified") {
      console.log(`FAIL ${line}`);
      failed++;
    } else {
      console.log(`diff ${line}`);
    }
  });
}
process.exit(failed ? 1 : 0);
