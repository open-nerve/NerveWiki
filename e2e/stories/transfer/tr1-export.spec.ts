import { expectExpired, expectExported } from "../../fixtures/assert/transfer";
import { pngBytes, uploadAsset, utf8 } from "../../fixtures/assets";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { archiveAt, downloadArchive, endedJob, listJobs, startExport } from "../../fixtures/transfer";
import { newTeam } from "../../fixtures/workspaces";
import type { ZipEntry } from "../../fixtures/zip";

// TR1, a notebook and a subtree exported as Obsidian vaults (M7 design 9;
// M7/P5 design 3.16): each export a job that runs in the background, its
// archive downloaded at the address the server signed, without a token. A
// page with content is its file, its children in its folder; a page without
// content that has children is its folder, unless a link leads to it; an
// attachment is its file in its page's folder, stored as it is; a folder
// that would be a sibling's file is renamed, and the report says so;
// meta.json holds the order. An export that succeeds expires its
// starter's earlier one in the notebook.

/** The meta.json of an archive, as much as the story reads. */
interface Meta {
  format: number;
  notebook: { id: string; name: string };
  root: { id: string; name: string } | null;
  nodes: { path: string; kind: string; id: string; sort_order: number }[];
}

/** The entry named name of entries. */
function entry(entries: ZipEntry[], name: string): ZipEntry {
  const e = entries.find((x) => x.name === name);
  if (!e) {
    throw new Error(`no ${name} in ${entries.map((x) => x.name).join(", ")}`);
  }
  return e;
}

test("TR1 (API): a notebook and a subtree export as vaults, their pages, folders, linked folder-only page, attachments and renamed folder as written, meta.json last; the later export expires the earlier one", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const handbook = await createNotebook(api, pat, workspace.slug, "Handbook");
  const guide = await createPage(api, pat, handbook.id, "Guide", null, "# Guide\n\nSee [[Linked]].\n");
  const chapters = await createPage(api, pat, handbook.id, "Chapters");
  const one = await createPage(api, pat, handbook.id, "One", chapters.id, "one\n");
  const linked = await createPage(api, pat, handbook.id, "Linked");
  const two = await createPage(api, pat, handbook.id, "Two", linked.id, "two\n");
  const plan = await createPage(api, pat, handbook.id, "Plan", null, "plan\n");
  const planFolder = await createPage(api, pat, handbook.id, "Plan.md");
  const step = await createPage(api, pat, handbook.id, "Step", planFolder.id, "step\n");
  const notesBytes = utf8("Notes at the root.\n");
  const diagram = await uploadAsset(api, pat, handbook.id, { name: "diagram.png", bytes: pngBytes }, guide.id);
  const notes = await uploadAsset(api, pat, handbook.id, { name: "notes.txt", bytes: notesBytes });

  const whole = await endedJob(api, pat, (await startExport(api, pat, handbook.id)).id);
  expect([whole.state, whole.name, whole.root_id, whole.client]).toEqual(["succeeded", "Handbook", null, "api"]);
  expect(whole.report).toEqual({
    failure: null,
    counts: { pages: 8, attachments: 2, renamed: 1, missing: 0, skipped: 0 },
  });
  expect(whole.problems).toEqual([{ path: "Plan.md/", code: "renamed", to: "Plan.md 2/" }]);
  if (!whole.download || whole.result_bytes === null) {
    throw new Error("a succeeded export without its archive");
  }
  const { entries, bytes } = await archiveAt(nervewiki.baseURL, whole.download.url);
  // The pages, in the vault's order; then the attachments' files; then meta.json.
  expect(entries.map((e) => e.name)).toEqual([
    "Handbook/Guide.md",
    "Handbook/Chapters/",
    "Handbook/Chapters/One.md",
    "Handbook/Linked.md",
    "Handbook/Linked/Two.md",
    "Handbook/Plan.md",
    "Handbook/Plan.md 2/",
    "Handbook/Plan.md 2/Step.md",
    "Handbook/Guide/diagram.png",
    "Handbook/notes.txt",
    "Handbook/.nerve/meta.json",
  ]);
  expect(entries.every((e) => e.utf8)).toBe(true);
  for (const [name, content] of [
    ["Handbook/Guide.md", "# Guide\n\nSee [[Linked]].\n"],
    ["Handbook/Chapters/One.md", "one\n"],
    ["Handbook/Linked.md", ""],
    ["Handbook/Linked/Two.md", "two\n"],
    ["Handbook/Plan.md", "plan\n"],
    ["Handbook/Plan.md 2/Step.md", "step\n"],
    ["Handbook/notes.txt", "Notes at the root.\n"],
  ] as const) {
    expect(entry(entries, name).data.toString("utf8"), name).toBe(content);
  }
  const png = entry(entries, "Handbook/Guide/diagram.png");
  expect([png.method, Buffer.compare(png.data, Buffer.from(pngBytes))]).toEqual([0, 0]);
  expect(entry(entries, "Handbook/Guide.md").method).toBe(8);
  const meta = JSON.parse(entry(entries, "Handbook/.nerve/meta.json").data.toString("utf8")) as Meta;
  expect([meta.format, meta.notebook, meta.root]).toEqual([1, { id: handbook.id, name: "Handbook" }, null]);
  expect(meta.nodes.map((n) => [n.path, n.kind, n.id])).toEqual([
    ["Guide.md", "page", guide.id],
    ["Guide/diagram.png", "asset", diagram.id],
    ["Chapters/", "page", chapters.id],
    ["Chapters/One.md", "page", one.id],
    ["Linked.md", "page", linked.id],
    ["Linked/Two.md", "page", two.id],
    ["Plan.md", "page", plan.id],
    ["Plan.md 2/", "page", planFolder.id],
    ["Plan.md 2/Step.md", "page", step.id],
    ["notes.txt", "asset", notes.id],
  ]);
  expect(whole.result_bytes).toBe(bytes);
  await expectExported(db, nervewiki.storageDir, whole.id, {
    creatorId: adminId,
    client: "api",
    root: null,
    nodes: 10,
    counts: { pages: 8, attachments: 2, renamed: 1, missing: 0 },
    bytes,
  });

  // A subtree is at the vault's root, named after its page; no link of it
  // leads to its root, which is then its folder alone.
  const sub = await endedJob(api, pat, (await startExport(api, pat, handbook.id, linked.id)).id);
  expect([sub.state, sub.name, sub.root_id]).toEqual(["succeeded", "Linked", linked.id]);
  if (!sub.download) {
    throw new Error("a succeeded export without its archive");
  }
  const subtree = await archiveAt(nervewiki.baseURL, sub.download.url);
  expect(subtree.entries.map((e) => e.name)).toEqual([
    "Linked/Linked/",
    "Linked/Linked/Two.md",
    "Linked/.nerve/meta.json",
  ]);
  const subMeta = JSON.parse(entry(subtree.entries, "Linked/.nerve/meta.json").data.toString("utf8")) as Meta;
  expect([subMeta.root, subMeta.nodes.map((n) => n.path)]).toEqual([
    { id: linked.id, name: "Linked" },
    ["Linked/", "Linked/Two.md"],
  ]);
  await expectExported(db, nervewiki.storageDir, sub.id, {
    creatorId: adminId,
    client: "api",
    root: linked.id,
    nodes: 2,
    counts: { pages: 2, attachments: 0, renamed: 0, missing: 0 },
    bytes: subtree.bytes,
  });

  // The later export expired the earlier: its archive is gone, its address
  // not found; the list shows both, the newest first.
  await expectExpired(db, nervewiki.storageDir, whole.id);
  expect((await downloadArchive(nervewiki.baseURL, whole.download.url)).status).toBe(404);
  expect((await listJobs(api, pat, handbook.id)).map((j) => [j.id, j.state, j.download === null])).toEqual([
    [sub.id, "succeeded", false],
    [whole.id, "expired", true],
  ]);
  // An address with one character changed is not found.
  const url = sub.download.url;
  const tampered = url.slice(0, -1) + (url.endsWith("A") ? "B" : "A");
  expect((await downloadArchive(nervewiki.baseURL, tampered)).status).toBe(404);
});
