// Compares the rename/ fixtures with how a real Obsidian rewrites links as a file is renamed or moved, over the
// Chrome DevTools Protocol.
//
//   node verify-rename.mjs prepare <workdir>        a scratch vault and an isolated user-data dir
//   node verify-rename.mjs check <workdir> [port]   rename in Obsidian case by case, and compare (default port 9333)
//
// A page A is A.md, its children are in A/: renaming or moving a page with children is two renames in Obsidian, of
// A.md and then of A/. An attachment is a file of its name, an image a real one, and renaming or moving it is one
// rename (M7/P3 design 4.8); the vault shows every type of file. Obsidian rewrites links only through app.fileManager.renameFile, and without asking only with
// "Automatically update internal links" on; the check turns it on, with the default (shortest) link format. Each case
// empties the vault and builds its pages in it again. Obsidian-verified cases must match; nerve-defined ones only
// report how they differ.
import { existsSync, readFileSync, readdirSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const dir = join(dirname(fileURLToPath(import.meta.url)), "..", "rename");
const [cmd, workArg, portArg] = process.argv.slice(2);
if (!["prepare", "check"].includes(cmd) || !workArg) {
  console.error("usage: node verify-rename.mjs prepare|check <workdir> [port]");
  process.exit(2);
}
const work = resolve(workArg);
// A 7×5 PNG, base64, for the attachments that are images; any other's bytes are a few of text.
const png = "iVBORw0KGgoAAAANSUhEUgAAAAcAAAAFCAYAAACJmvbYAAAAEklEQVR42mM4YWPzHxdmGABJADoKTp7oONgaAAAAAElFTkSuQmCC";
const isImage = (name) => /\.(png|jpe?g|gif|webp|bmp|avif)$/i.test(name);

if (cmd === "prepare") {
  // It empties work: only a directory it made before, or none.
  if (existsSync(work) && readdirSync(work).length > 0 && !existsSync(join(work, "userdata", "obsidian.json"))) {
    console.error(`${work} is not empty and was not prepared by this script: choose another directory`);
    process.exit(2);
  }
  rmSync(work, { recursive: true, force: true });
  mkdirSync(join(work, "vault"), { recursive: true });
  mkdirSync(join(work, "userdata"));
  const vaults = { nwikirename00001: { path: join(work, "vault"), ts: Date.now(), open: true } };
  writeFileSync(join(work, "userdata", "obsidian.json"), JSON.stringify({ vaults, updateDisabled: true }));
  const args = `--user-data-dir="${join(work, "userdata")}" --remote-debugging-port=9333`;
  console.log(`Start an isolated Obsidian (it does not touch your own vaults), then run "check":`);
  console.log(`  macOS: open -n -g -a Obsidian --args ${args}`);
  console.log(`  Linux: obsidian ${args}`);
  process.exit(0);
}

const cases = readdirSync(dir)
  .filter((f) => f.endsWith(".json"))
  .toSorted()
  .map((f) => {
    const c = JSON.parse(readFileSync(join(dir, f), "utf8"));
    const base = f.slice(0, -5);
    const page = c.page ?? "src";
    const assets = c.assets ?? [];
    return Object.assign(c, {
      name: base,
      page,
      assets,
      pages: c.pages ?? (assets.includes(c.from) ? [page] : [c.from, page]),
      md: readFileSync(join(dir, `${base}.md`), "utf8"),
      out: readFileSync(join(dir, `${base}.out.md`), "utf8"),
    });
  });

async function evaluate(port, expression) {
  const pages = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
  const ws = new WebSocket(pages.find((p) => p.type === "page").webSocketDebuggerUrl);
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

// One case in Obsidian: its pages, its rename or move, and the page's bytes after.
function caseExpression(c) {
  const files = Object.fromEntries(
    c.pages.map((p) => {
      const aliases = c.aliases?.[p];
      const body = p === c.page ? c.md : aliases ? `---\naliases: ${JSON.stringify(aliases)}\n---\n` : "";
      return [`${p}.md`, body];
    })
  );
  const withChildren = [...c.pages, ...c.assets].some((p) => p.startsWith(`${c.from}/`));
  const renames = c.assets.includes(c.from)
    ? [[c.from, c.to]]
    : [[`${c.from}.md`, `${c.to}.md`], ...(withChildren ? [[c.from, c.to]] : [])];
  const binaries = Object.fromEntries(c.assets.map((a) => [a, isImage(a) ? png : btoa("not really\n")]));
  const moved = c.page === c.from || c.page.startsWith(`${c.from}/`);
  const after = `${moved ? c.to + c.page.slice(c.from.length) : c.page}.md`;
  return `(async () => {
    // It empties the vault: only the scratch one.
    if (app.vault.adapter.basePath !== ${JSON.stringify(join(work, "vault"))}) throw new Error("not the scratch vault: " + app.vault.adapter.basePath);
    const settle = async () => {
      for (let i = 0; i < 200; i++) {
        const files = app.vault.getMarkdownFiles();
        if (files.every((f) => app.metadataCache.getFileCache(f) && f.path in app.metadataCache.resolvedLinks)) break;
        await new Promise((r) => setTimeout(r, 50));
      }
      await new Promise((r) => app.metadataCache.onCleanCache(r));
      await new Promise((r) => setTimeout(r, 300));
      await new Promise((r) => app.metadataCache.onCleanCache(r));
    };
    const folder = async (path) => {
      let at = "";
      for (const part of path.split("/").filter((p) => p !== "")) {
        at = at === "" ? part : at + "/" + part;
        if (!app.vault.getAbstractFileByPath(at)) await app.vault.createFolder(at);
      }
    };
    for (const child of [...app.vault.getRoot().children]) await app.vault.delete(child, true);
    await settle();
    app.vault.setConfig("alwaysUpdateLinks", true);
    app.vault.setConfig("newLinkFormat", "shortest");
    app.vault.setConfig("useMarkdownLinks", false);
    app.vault.setConfig("showUnsupportedFiles", true);
    for (const [path, b64] of Object.entries(${JSON.stringify(binaries)})) {
      await folder(path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "");
      await app.vault.createBinary(path, Uint8Array.from(atob(b64), (ch) => ch.charCodeAt(0)).buffer);
    }
    for (const [path, body] of Object.entries(${JSON.stringify(files)})) {
      await folder(path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "");
      await app.vault.create(path, body);
    }
    await settle();
    for (const [from, to] of ${JSON.stringify(renames)}) {
      await folder(to.includes("/") ? to.slice(0, to.lastIndexOf("/")) : "");
      await app.fileManager.renameFile(app.vault.getAbstractFileByPath(from), to);
      await settle();
    }
    return await app.vault.adapter.read(${JSON.stringify(after)});
  })()`;
}

const port = portArg ?? 9333;
let version;
try {
  version = await evaluate(port, `require('electron').ipcRenderer.sendSync('version')`);
} catch {
  version = "?";
}
console.log(`Obsidian ${version}, ${cases.length} cases`);
let failed = 0;
for (const c of cases) {
  // oxlint-disable-next-line no-await-in-loop -- one case after another, in the one vault
  const got = await evaluate(port, caseExpression(c));
  if (got === c.out) {
    if (c.source === "nerve-defined") console.log(`same ${c.name}: Obsidian agrees, reconsider nerve-defined`);
    continue;
  }
  const line = `${c.name}:\n  Obsidian ${JSON.stringify(got)}\n  the case ${JSON.stringify(c.out)}`;
  if (c.source === "obsidian-verified") {
    console.log(`FAIL ${line}`);
    failed++;
  } else {
    console.log(`diff ${line}`);
  }
}
process.exit(failed ? 1 : 0);
