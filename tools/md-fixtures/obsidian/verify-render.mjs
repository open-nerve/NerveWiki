// Compares the render fixtures with what a real Obsidian's reading view shows, over the Chrome DevTools Protocol.
//
//   node verify-render.mjs prepare <workdir>        build a scratch vault and an isolated user-data dir
//   node verify-render.mjs check <workdir> [port]   read Obsidian's reading view and compare (default port 9334)
//
// A fixture's "rendered" is its reading view as text: "¶" between blocks, "⏎" a line break, the one line break at a
// block's end left out (it shows none), runs of ASCII white space one space; an attachment's embedded image, audio or
// video as "⟨img text size⟩", "⟨audio text⟩", "⟨video text size⟩", its text and size those it is written with (the
// embed's alt, width and height), the size "w", "w×h" or "×h" (M7/P3 design 5.8). A fixture's "assets" are files at
// the vault's root, which "prepare" writes: a real image for a PNG. Obsidian-verified fixtures must match;
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

// PNG is a 7×5 image; the other attachments' bytes do not matter, as Obsidian shows a file by its extension.
const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAcAAAAFCAYAAACJmvbYAAAAEklEQVR42mM4YWPzHxdmGABJADoKTp7oONgaAAAAAElFTkSuQmCC",
  "base64"
);
const assetsOf = (name) => {
  try {
    return JSON.parse(readFileSync(join(renderDir, name.replace(/\.md$/, ".json")), "utf8")).assets ?? [];
  } catch {
    return [];
  }
};

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
  for (const a of new Set(names.flatMap(assetsOf))) {
    writeFileSync(join(vault, a), a.toLowerCase().endsWith(".png") ? PNG : Buffer.alloc(64));
  }
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

// DUMP opens each fixture in the reading view, in a tab emptied before each so that no earlier file's view is read,
// in the scratch vault only, with strict line breaks off (Obsidian's default), and writes its blocks as text, as the
// server's test reads its HTML.
const DUMP = `(async () => {
  if (app.vault.adapter.basePath !== ${JSON.stringify(vault)}) throw new Error("not the scratch vault: " + app.vault.adapter.basePath);
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  await new Promise((r) => app.workspace.onLayoutReady(r));
  const names = ${JSON.stringify(names)};
  const assets = ${JSON.stringify([...new Set(names.flatMap(assetsOf))])};
  for (let i = 0; i < 100 && [...names, ...assets].some((n) => !app.vault.getFileByPath(n)); i++) await sleep(100);
  app.vault.setConfig("strictLineBreaks", false);
  let version;
  try { version = require('electron').ipcRenderer.sendSync('version'); } catch { version = navigator.userAgent.match(/obsidian\\/([\\d.]+)/)?.[1]; }
  const BLOCK = new Set(["P", "DIV", "LI", "UL", "OL", "BLOCKQUOTE", "H1", "H2", "H3", "H4", "H5", "H6", "PRE", "TABLE", "THEAD", "TBODY", "TR", "TD", "TH", "HR", "DETAILS", "SUMMARY", "SECTION"]);
  const SKIP = ["mod-header", "mod-footer", "markdown-preview-pusher", "inline-title", "metadata-container", "mod-frontmatter", "embedded-backlinks", "math"];
  const MEDIA = { "image-embed": "img", "audio-embed": "audio", "video-embed": "video" };
  // An embed as its kind, its text and the size it is written with: its alt, width and height.
  const embed = (kind, n) => {
    const w = n.getAttribute("width") ?? "", h = n.getAttribute("height") ?? "";
    const size = w + (h === "" ? "" : "×" + h);
    return "⟨" + [kind, n.getAttribute("alt") ?? "", size].filter((p) => p !== "").join(" ") + "⟩";
  };
  const flatten = (roots) => {
    let out = "";
    const walk = (n) => {
      if (n.nodeType === 3) { out += n.nodeValue; return; }
      if (n.nodeType !== 1 || n.tagName === "svg" || n.tagName === "BUTTON" || SKIP.some((c) => n.classList.contains(c))) return;
      if (n.tagName === "BR") { out += "⏎"; return; }
      const media = Object.keys(MEDIA).find((c) => n.classList.contains(c));
      if (media !== undefined && n.classList.contains("internal-embed")) { out += embed(MEDIA[media], n); return; }
      if (n.tagName === "IMG") { out += embed("img", n); return; }
      const block = BLOCK.has(n.tagName);
      if (block) out += "¶";
      for (const c of n.childNodes) walk(c);
      if (block) out += "¶";
    };
    for (const r of roots) walk(r);
    return out.replace(/[ \\t\\n\\f\\r]+/g, " ").replace(/ ?([¶⏎]) ?/g, "$1").replace(/¶+/g, "¶").replace(/⏎¶/g, "¶").replace(/^¶|¶$/g, "");
  };
  const out = { version, files: {} };
  const leaf = app.workspace.getLeaf("tab");
  for (const name of names) {
    const f = app.vault.getFileByPath(name);
    if (!f) continue;
    await leaf.setViewState({ type: "empty" });
    await leaf.openFile(f, { state: { mode: "preview" } });
    let text = "", last = null;
    for (let i = 0; i < 50; i++) {
      await sleep(100);
      const section = leaf.view.file === f && leaf.view.getMode?.() === "preview" ? leaf.view.previewMode.containerEl.querySelector(".markdown-preview-section") : null;
      text = section ? flatten([...section.children]) : "";
      if (text !== "" && text === last) break;
      last = text;
    }
    out.files[name] = text;
  }
  leaf.detach();
  return JSON.stringify(out);
})()`;

const got = JSON.parse(await evaluate(DUMP));
console.log(`Obsidian ${got.version}, ${names.length} cases`);
let failed = 0;
for (const f of names) {
  const shown = got.files[f];
  if (shown === undefined) {
    console.log(`FAIL ${f}: not in the vault`);
    failed++;
    continue;
  }
  let want;
  try {
    want = JSON.parse(readFileSync(join(renderDir, f.replace(/\.md$/, ".json")), "utf8"));
  } catch {
    console.log(`FAIL ${f}: no JSON; Obsidian shows ${JSON.stringify(shown)}`);
    failed++;
    continue;
  }
  if (shown === want.rendered) {
    if (want.source === "nerve-defined") console.log(`same ${f}: Obsidian agrees, reconsider nerve-defined`);
    continue;
  }
  const line = `${f}:\n  Obsidian ${JSON.stringify(shown)}\n  the case ${JSON.stringify(want.rendered)}`;
  if (want.source === "obsidian-verified") {
    console.log(`FAIL ${line}`);
    failed++;
  } else {
    console.log(`diff ${line}`);
  }
}
process.exit(failed ? 1 : 0);
