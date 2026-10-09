// Compares the exports of the resolve/ fixtures with how a real Obsidian resolves their links, over the Chrome DevTools
// Protocol (M7/P5 design 3.15).
//
//   node verify-export.mjs prepare <workdir> <vaults>   one vault per exported case, an isolated user-data dir
//   node verify-export.mjs check <workdir> [port]       read Obsidian's resolved links and compare (default port 9333)
//
// <vaults> is where the server's test wrote each case's archive and its links, as this system resolves them, by
// their paths in the vault: NWIKI_EXPORT_VAULTS=<vaults> go test ./internal/bootstrap -run
// TestEveryResolutionCaseExportsAsAVault. Each archive is unpacked as a vault, its root folder left out, the vault
// showing every type of file ("Detect all file extensions"). Obsidian-verified cases must match; nerve-defined ones
// only report how they differ.
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { inflateRawSync } from "node:zlib";

const [cmd, workArg, thirdArg] = process.argv.slice(2);
if (!["prepare", "check"].includes(cmd) || !workArg || (cmd === "prepare" && !thirdArg)) {
  console.error("usage: node verify-export.mjs prepare <workdir> <vaults> | check <workdir> [port]");
  process.exit(2);
}
const work = resolve(workArg);
const casesDir = join(work, "cases");

// unzip reads a zip archive's entries from its central directory: each name, and its bytes, or null for a folder.
// Only the store and deflate methods, and archives under 4 GiB, which the exports of the cases are.
function unzip(buf) {
  let end = buf.length - 22;
  while (end >= 0 && buf.readUInt32LE(end) !== 0x06054b50) end--;
  if (end < 0) throw new Error("not a zip archive");
  const entries = [];
  let at = buf.readUInt32LE(end + 16);
  for (let i = buf.readUInt16LE(end + 10); i > 0; i--) {
    if (buf.readUInt32LE(at) !== 0x02014b50) throw new Error("a broken central directory");
    const method = buf.readUInt16LE(at + 10);
    const size = buf.readUInt32LE(at + 20);
    const nameLength = buf.readUInt16LE(at + 28);
    const local = buf.readUInt32LE(at + 42);
    const name = buf.toString("utf8", at + 46, at + 46 + nameLength);
    at += 46 + nameLength + buf.readUInt16LE(at + 30) + buf.readUInt16LE(at + 32);
    const start = local + 30 + buf.readUInt16LE(local + 26) + buf.readUInt16LE(local + 28);
    const raw = buf.subarray(start, start + size);
    if (method !== 0 && method !== 8) throw new Error(`${name}: compression method ${method}`);
    entries.push({ name, data: name.endsWith("/") ? null : method === 0 ? raw : inflateRawSync(raw) });
  }
  return entries;
}

if (cmd === "prepare") {
  const vaults = resolve(thirdArg);
  // It empties work: only a directory it made before, or none.
  if (existsSync(work) && readdirSync(work).length > 0 && !existsSync(join(work, "userdata", "obsidian.json"))) {
    console.error(`${work} is not empty and was not prepared by this script: choose another directory`);
    process.exit(2);
  }
  rmSync(work, { recursive: true, force: true });
  mkdirSync(join(work, "userdata"), { recursive: true });
  mkdirSync(casesDir);
  const names = readdirSync(vaults)
    .filter((f) => f.endsWith(".zip"))
    .toSorted()
    .map((f) => f.slice(0, -4));
  if (names.length === 0) {
    console.error(`${vaults} holds no archive: run the server's test with NWIKI_EXPORT_VAULTS=${vaults} first`);
    process.exit(2);
  }
  const opened = {};
  for (const name of names) {
    const vault = join(work, name);
    for (const { name: entry, data } of unzip(readFileSync(join(vaults, `${name}.zip`)))) {
      const segments = entry.split("/");
      if (data === null) segments.pop(); // a folder's name ends in "/"
      if (segments.some((s) => s === "" || s === "." || s === "..")) throw new Error(`${name}: the entry ${entry}`);
      if (segments.length < 2) continue; // the root folder itself
      // The archive's root folder is the notebook's: the vault is what it holds.
      const path = join(vault, ...segments.slice(1));
      if (data === null) {
        mkdirSync(path, { recursive: true });
      } else {
        mkdirSync(dirname(path), { recursive: true });
        writeFileSync(path, data);
      }
    }
    mkdirSync(join(vault, ".obsidian"), { recursive: true });
    writeFileSync(join(vault, ".obsidian", "app.json"), JSON.stringify({ showUnsupportedFiles: true }));
    copyFileSync(join(vaults, `${name}.json`), join(casesDir, `${name}.json`));
    opened[
      name
        .replace(/[^0-9a-z]/g, "")
        .slice(0, 16)
        .padEnd(16, "0")
    ] = { path: vault, ts: Date.now(), open: true };
  }
  writeFileSync(join(work, "userdata", "obsidian.json"), JSON.stringify({ vaults: opened, updateDisabled: true }));
  const args = `--user-data-dir="${join(work, "userdata")}" --remote-debugging-port=9333`;
  console.log(
    `${names.length} vaults. Start an isolated Obsidian (it does not touch your own vaults; it opens one window a vault), then run "check":`
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

// Each file's links, resolved and not: Obsidian sets both maps of a file together, once it resolved its links; until
// every file has its entry, it has not.
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

const cases = readdirSync(casesDir)
  .filter((f) => f.endsWith(".json"))
  .toSorted()
  .map((f) => Object.assign(JSON.parse(readFileSync(join(casesDir, f), "utf8")), { name: f.slice(0, -5) }));
const port = thirdArg ?? 9333;
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
let links = 0;
for (const c of cases) {
  const vault = byVault[join(work, c.name)];
  if (!vault) {
    console.log(`FAIL ${c.name}: its vault is not open`);
    failed++;
    continue;
  }
  for (const l of c.links) {
    links++;
    const got = vault.resolved[l.query];
    const unresolved = vault.unresolved[l.query];
    if (got === undefined || got.length + unresolved.length !== 1) {
      console.log(
        `FAIL ${c.name} ${l.query} ${l.link}: Obsidian took ${got === undefined ? "no" : got.length + unresolved.length} links`
      );
      failed++;
      continue;
    }
    const gotPath = got.length === 1 ? got[0] : null;
    if (gotPath === l.nerve) {
      if (l.source === "nerve-defined")
        console.log(`same ${c.name} ${l.query} ${l.link}: Obsidian agrees, reconsider nerve-defined`);
      continue;
    }
    const line = `${c.name} ${l.query} ${l.link}: Obsidian ${gotPath}, this system ${l.nerve}`;
    if (l.source === "obsidian-verified") {
      console.log(`FAIL ${line}`);
      failed++;
    } else {
      console.log(`diff ${line}`);
    }
  }
}
console.log(`${links} links, ${failed} failed`);
process.exit(failed ? 1 : 0);
