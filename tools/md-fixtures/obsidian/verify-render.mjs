// Compares the render fixtures with what a real Obsidian's reading view shows, over the Chrome DevTools Protocol.
//
//   node verify-render.mjs prepare <workdir>        build a scratch vault and an isolated user-data dir
//   node verify-render.mjs check <workdir> [port]   read Obsidian's reading view and compare (default port 9334)
//
// A fixture's "rendered" is its reading view as text: "¶" between blocks, "⏎" a line break, a line break at a
// block's end left out (it shows none), runs of white space one space. Obsidian-verified fixtures must match;
// nerve-defined ones only report how they differ.
import { existsSync, readFileSync, readdirSync, writeFileSync, mkdirSync, rmSync, copyFileSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const renderDir = join(dirname(fileURLToPath(import.meta.url)), "..", "render");
const [cmd, workArg, portArg] = process.argv.slice(2);
if (!["prepare", "check"].includes(cmd) || !workArg) {
  console.error("usage: node verify-render.mjs prepare|check <workdir> [port]");
  process.exit(2);
}
const work = resolve(workArg);
const vault = join(work, "vault");
const port = portArg ?? "9334";
const names = readdirSync(renderDir)
  .filter((f) => f.endsWith(".md"))
  .toSorted();

if (cmd === "prepare") {
  // It empties work: only a directory it made before, or none.
  if (existsSync(work) && readdirSync(work).length > 0 && !existsSync(join(work, "userdata", "obsidian.json"))) {
    console.error(`${work} is not empty and was not prepared by this script: choose another directory`);
    process.exit(2);
  }
  rmSync(work, { recursive: true, force: true });
  mkdirSync(vault, { recursive: true });
  mkdirSync(join(work, "userdata"));
  for (const f of names) copyFileSync(join(renderDir, f), join(vault, f));
  const vaults = { nwikirender0001: { path: vault, ts: Date.now(), open: true } };
  writeFileSync(join(work, "userdata", "obsidian.json"), JSON.stringify({ vaults, updateDisabled: true }));
  const args = `--user-data-dir="${join(work, "userdata")}" --remote-debugging-port=${port}`;
  console.log(`Start an isolated Obsidian (it does not touch your own vaults), then run "check":`);
  console.log(`  macOS: open -n -g -a Obsidian --args ${args}`);
  console.log(`  Linux: obsidian ${args}`);
  process.exit(0);
}

async function evaluate(expression) {
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

// DUMP opens each file in the reading view and writes its blocks as text, as server's tests do with its HTML.
const DUMP = `(async () => {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  for (let i = 0; i < 100 && app.vault.getMarkdownFiles().length < ${names.length}; i++) await sleep(100);
  let version;
  try { version = require('electron').ipcRenderer.sendSync('version'); } catch { version = navigator.userAgent.match(/obsidian\\/([\\d.]+)/)?.[1]; }
  const BLOCK = new Set(["P", "DIV", "LI", "UL", "OL", "BLOCKQUOTE", "H1", "H2", "H3", "H4", "H5", "H6", "PRE", "TABLE", "THEAD", "TBODY", "TR", "TD", "TH", "HR", "DETAILS", "SUMMARY", "SECTION"]);
  const SKIP = ["mod-header", "mod-footer", "markdown-preview-pusher", "inline-title", "metadata-container", "embedded-backlinks"];
  const flatten = (roots) => {
    let out = "";
    const walk = (n) => {
      if (n.nodeType === 3) { out += n.nodeValue; return; }
      if (n.nodeType !== 1 || n.tagName === "svg" || n.tagName === "BUTTON" || SKIP.some((c) => n.classList.contains(c))) return;
      if (n.tagName === "BR") { out += "⏎"; return; }
      const block = BLOCK.has(n.tagName);
      if (block) out += "¶";
      for (const c of n.childNodes) walk(c);
      if (block) out += "¶";
    };
    for (const r of roots) walk(r);
    return out.replace(/\\s+/g, " ").replace(/ ?([¶⏎]) ?/g, "$1").replace(/¶+/g, "¶").replace(/⏎¶/g, "¶").replace(/^¶|¶$/g, "");
  };
  const out = { version, files: {} };
  const leaf = app.workspace.getLeaf(false);
  for (const f of app.vault.getMarkdownFiles()) {
    await leaf.openFile(f, { state: { mode: "preview" } });
    let text = "", last = null;
    for (let i = 0; i < 50; i++) {
      await sleep(100);
      const section = leaf.view.previewMode?.containerEl.querySelector(".markdown-preview-section");
      text = section ? flatten([...section.children]) : "";
      if (text !== "" && text === last) break;
      last = text;
    }
    out.files[f.path] = text;
  }
  return JSON.stringify(out);
})()`;

const got = JSON.parse(await evaluate(DUMP));
console.log(`Obsidian ${got.version}`);
let failed = 0;
for (const f of names) {
  const shown = got.files[f];
  let want;
  try {
    want = JSON.parse(readFileSync(join(renderDir, f.replace(/\.md$/, ".json")), "utf8"));
  } catch {
    console.log(`${f}: (no JSON) Obsidian shows ${JSON.stringify(shown)}`);
    continue;
  }
  if (shown === want.rendered) {
    console.log(`${f}: ok`);
  } else if (want.source === "nerve-defined") {
    console.log(
      `${f}: nerve-defined, Obsidian shows ${JSON.stringify(shown)}, the fixture ${JSON.stringify(want.rendered)}`
    );
  } else {
    failed++;
    console.log(
      `${f}: MISMATCH: Obsidian shows ${JSON.stringify(shown)}, the fixture ${JSON.stringify(want.rendered)}`
    );
  }
}
process.exit(failed > 0 ? 1 : 0);
