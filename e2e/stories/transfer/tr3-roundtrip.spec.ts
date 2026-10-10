import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectImported, idsOf, treeOf } from "../../fixtures/assert/transfer";
import { pngBytes, uploadAsset, utf8 } from "../../fixtures/assets";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, moveNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { downloadArchive, endedJob, startExport, startImport } from "../../fixtures/transfer";
import { downloadFrom, exportWith, importWith, jobRow } from "../../fixtures/wiki-transfer";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// TR3, an export imported again (M7 design 9; M7/P6 design 3.18): TR1's
// notebook exported, its archive imported into another notebook, which
// holds the same tree, in the same order, the same contents and
// attachments, its links leading to the same pages; but for the folder the
// export renamed, which the import takes as the vault names it.

test("TR3 (API): TR1's notebook exports and imports into another as it was: its tree, order, contents and attachments; the folder the export renamed as the vault names it", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, adminId, workspace } = await newTeam(api, testInfo);
  const handbook = await createNotebook(api, pat, workspace.slug, "Handbook");
  const guide = await createPage(api, pat, handbook.id, "Guide", null, "# Guide\n\nSee [[Linked]].\n");
  const chapters = await createPage(api, pat, handbook.id, "Chapters");
  await createPage(api, pat, handbook.id, "One", chapters.id, "one\n");
  const linked = await createPage(api, pat, handbook.id, "Linked");
  await createPage(api, pat, handbook.id, "Two", linked.id, "two\n");
  await createPage(api, pat, handbook.id, "Plan", null, "plan\n");
  const planFolder = await createPage(api, pat, handbook.id, "Plan.md");
  await createPage(api, pat, handbook.id, "Step", planFolder.id, "step\n");
  const cafe = await createPage(api, pat, handbook.id, "Café", null, "café\n");
  await moveNode(api, pat, cafe.id, { parent_id: null, after_id: guide.id });
  await uploadAsset(api, pat, handbook.id, { name: "diagram.png", bytes: pngBytes }, guide.id);
  await uploadAsset(api, pat, handbook.id, { name: "notes.txt", bytes: utf8("Notes at the root.\n") });

  const exported = await endedJob(api, pat, (await startExport(api, pat, handbook.id)).id);
  expect([exported.state, exported.problems]).toEqual([
    "succeeded",
    [{ path: "Plan.md/", code: "renamed", to: "Plan.md 2/" }],
  ]);
  if (!exported.download) {
    throw new Error("a succeeded export without its archive");
  }
  const answer = await downloadArchive(nervewiki.baseURL, exported.download.url);
  expect(answer.status).toBe(200);
  const archive = Buffer.from(await answer.arrayBuffer());

  const copy = await createNotebook(api, pat, workspace.slug, "Copy");
  const job = await endedJob(api, pat, (await startImport(api, pat, copy.id, archive)).id);
  expect([job.state, job.report, job.problems]).toEqual([
    "succeeded",
    { failure: null, counts: { pages: 9, attachments: 2, renamed: 0, missing: 0, skipped: 0 } },
    [],
  ]);
  const before = await treeOf(db, nervewiki.storageDir, handbook.id);
  expect(before.map((e) => e.path)).toEqual([
    "Guide",
    "Guide/diagram.png",
    "Café",
    "Chapters",
    "Chapters/One",
    "Linked",
    "Linked/Two",
    "Plan",
    "Plan.md",
    "Plan.md/Step",
    "notes.txt",
  ]);
  // The copy is the same, but for the folder renamed.
  for (const e of before) {
    e.path = e.path.replace(/^Plan\.md\b/, "Plan.md 2");
  }
  expect(await treeOf(db, nervewiki.storageDir, copy.id)).toEqual(before);
  await expectImported(db, nervewiki.storageDir, job.id, {
    creatorId: adminId,
    client: "api",
    root: null,
    state: "succeeded",
    failure: null,
    done: 11,
    total: 11,
    counts: { pages: 9, attachments: 2, renamed: 0, skipped: 0 },
  });
  const ids = await idsOf(db, copy.id);
  await expectIndexedLinks(db, ids.get("Guide") ?? "", [
    { kind: "wikilink", property: null, target: "Linked", resolved: ids.get("Linked") ?? "" },
  ]);
});

test("TR3 (page): a notebook exported from its settings, its archive downloaded and imported from another's, is the same", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  // Each import runs as a job, waited for: slow runners take longer than a test is given.
  test.slow();
  const { tokens, adminId, pat, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  const handbook = await createNotebook(api, pat, workspace.slug, "Handbook");
  const guide = await createPage(api, pat, handbook.id, "Guide", null, "# Guide\n\nSee [[Linked]].\n");
  const linked = await createPage(api, pat, handbook.id, "Linked");
  await createPage(api, pat, handbook.id, "Two", linked.id, "two\n");
  const cafe = await createPage(api, pat, handbook.id, "Café", null, "café\n");
  await moveNode(api, pat, cafe.id, { parent_id: null, after_id: guide.id });
  await uploadAsset(api, pat, handbook.id, { name: "diagram.png", bytes: pngBytes }, guide.id);

  await page.goto(notebookPath(workspace.slug, handbook.id, "transfer"));
  await page.getByRole("button", { name: "Export the whole notebook" }).click();
  await exportWith(page, handbook.id, "Export Handbook?");
  const { name, archive } = await downloadFrom(page, jobRow(page, "Export of the whole notebook"));
  expect(name).toBe("Handbook.zip");

  const copy = await createNotebook(api, pat, workspace.slug, "Copy");
  await page.goto(notebookPath(workspace.slug, copy.id, "transfer"));
  const job = await importWith(page, copy.id, archive, { name });
  await expect(jobRow(page, "Import of Handbook.zip").getByText("Done", { exact: true })).toBeVisible({
    timeout: 15_000,
  });
  const before = await treeOf(db, nervewiki.storageDir, handbook.id);
  expect(before.map((e) => e.path)).toEqual(["Guide", "Guide/diagram.png", "Café", "Linked", "Linked/Two"]);
  expect(await treeOf(db, nervewiki.storageDir, copy.id)).toEqual(before);
  await expectImported(db, nervewiki.storageDir, job.id, {
    creatorId: adminId,
    client: "web",
    root: null,
    state: "succeeded",
    failure: null,
    done: 5,
    total: 5,
    counts: { pages: 4, attachments: 1, renamed: 0, skipped: 0 },
  });
});
