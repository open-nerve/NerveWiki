import type { ApiClient, Asset, TransferJob } from "@nervewiki/api-client";

import { expectAssetsRestored } from "../../fixtures/assert/asset";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectSubtreeRestored, expectTrashRoot } from "../../fixtures/assert/page";
import { expectBackgroundJob, idsOf } from "../../fixtures/assert/transfer";
import { download, getAsset, listAssets, pngBytes } from "../../fixtures/assets";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, holdNotebookRow, startImport, startTrashRestore } from "../../fixtures/transfer";
import { restoreNode, restoreWith, trashListed, trashStatus } from "../../fixtures/trash";
import { choosePageAction, pageHeading, pageTree, subpages, treeTitles, wikiPagePath } from "../../fixtures/wiki-pages";
import { jobRow, openReport } from "../../fixtures/wiki-transfer";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// TB2, a trash item restored in the background (M8 design 4.6, 4.10,
// section 9; M8/P4 design 4.5): an item of more pages than one restore
// brings back is too large to restore here; restored in the background, a
// job puts it back batch by batch, its first batch waiting while the
// notebook's row is held, its progress there all the same. Done, the item
// is back where it was, with its attachment, its file and its links, in
// one changeset of its batches' items.

/** How many notes Big holds: with Big, more pages than one restore brings back (MaxUnitPages, 100). */
const notes = 101;

/** Big's nodes: the page, its notes and its attachment, more than a batch writes (100). */
const nodes = notes + 2;

/** How long a job of the story is given, longer than a job's default wait. */
const jobTime = 30_000;

/** The name of the note n of Big. */
function note(n: number): string {
  return `Note ${String(n).padStart(3, "0")}`;
}

/**
 * A vault of Big/, a folder: its notes, each linking to the next, the last to the first, and photo.png, which the first
 * embeds.
 */
function vault(): Buffer {
  return zipOf([
    ...Array.from({ length: notes }, (_, at) => ({
      name: `Big/${note(at + 1)}.md`,
      data: `${at === 0 ? "![[photo.png]] " : ""}[[${note(((at + 1) % notes) + 1)}]]\n`,
    })),
    { name: "Big/photo.png", data: pngBytes, method: 0 },
  ]);
}

/** Big imported: its id, the ids of its nodes, its first, second and last notes', and its attachment. */
interface Big {
  id: string;
  ids: string[];
  first: string;
  second: string;
  last: string;
  photo: Asset;
}

/** Imports the vault into the notebook notebookId with pat, at its root after its pages, and returns Big there. */
async function importBig(api: ApiClient, db: Database, pat: string, notebookId: string): Promise<Big> {
  const imported = await endedJob(api, pat, (await startImport(api, pat, notebookId, vault())).id, jobTime);
  expect(imported.state).toBe("succeeded");
  const ids = new Map([...(await idsOf(db, notebookId))].filter(([path]) => path === "Big" || path.startsWith("Big/")));
  const id = ids.get("Big") ?? "";
  const [photo] = await listAssets(api, pat, notebookId, id);
  if (photo === undefined) {
    throw new Error("Big imported without photo.png");
  }
  expect(ids.size).toBe(nodes);
  const of = (n: number) => ids.get(`Big/${note(n)}`) ?? "";
  return { id, ids: [...ids.values()], first: of(1), second: of(2), last: of(notes), photo };
}

/**
 * transfer_jobs, changesets, changeset_items, nodes, asset_blobs, page_links and the store at storageDir: the job of
 * byId from client restored Big in the background where it was, at the notebook's top level, with its attachment and
 * its file, and its links indexed again, as one changeset, the only restore of the notebook notebookId, of an item of
 * each node, written by its two batches, each at its time: a batch writes 100 nodes at most.
 */
async function expectBigRestored(
  db: Database,
  storageDir: string,
  big: Big,
  { notebookId, job, byId, client }: { notebookId: string; job: string; byId: string; client: string }
): Promise<void> {
  const restore = await expectBackgroundJob(db, job, {
    kind: "trash_restore",
    creatorId: byId,
    client,
    root: big.id,
    target: null,
    nodes,
    counts: { pages: notes + 1, attachments: 1 },
    placed: { parent_id: null, reason: "original" },
  });
  const batches = await db.query<{ items: number }>(
    `SELECT count(*)::int AS items FROM changeset_items WHERE changeset_id = $1
      GROUP BY created_at ORDER BY created_at`,
    [restore]
  );
  expect(batches.map((batch) => batch.items)).toEqual([100, nodes - 100]);
  expect(
    await db.query("SELECT id FROM changesets WHERE notebook_id = $1 AND kind = 'trash_restore'", [notebookId])
  ).toEqual([{ id: restore }]);
  await expectSubtreeRestored(db, big.id, big.ids, byId, { parentId: null, name: "Big" }, { kind: "trash_restore" });
  await expectAssetsRestored(db, storageDir, [big.photo]);
  // Note 001's embed of photo.png crosses the batches: Big and Note 001 to 099 in the first, photo.png in the second.
  await expectIndexedLinks(db, big.first, [
    { kind: "embed", property: null, target: "photo.png", resolved: big.photo.id },
    { kind: "wikilink", property: null, target: note(2), resolved: big.second },
  ]);
  await expectIndexedLinks(db, big.last, [{ kind: "wikilink", property: null, target: note(1), resolved: big.first }]);
}

test("TB2 (API): an item of more pages than a restore brings back is refused as too large; restored in the background, it is back where it was in two batches, with its attachment, its file and its links", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const big = await importBig(api, db, pat, notebook.id);
  expect((await deleteNode(api, pat, big.id)).response.status).toBe(204);
  await expectTrashRoot(db, big.id, { pages: notes + 1, assets: 1, height: 2 }, { alone: true });

  const refused = await restoreNode(api, pat, big.id);
  expect([refused.response.status, refused.error?.code]).toEqual([422, "page.restore_too_large"]);
  const queued = await startTrashRestore(api, pat, notebook.id, big.id);
  expect([queued.kind, queued.state, queued.name, queued.root_id, queued.target_changeset_id, queued.client]).toEqual([
    "trash_restore",
    "queued",
    "Big",
    big.id,
    null,
    "api",
  ]);
  const job = await endedJob(api, pat, queued.id, jobTime);
  expect([job.state, job.progress, job.changeset_id]).toEqual([
    "succeeded",
    { done: nodes, total: nodes },
    queued.changeset_id,
  ]);
  expect([job.report?.counts.pages, job.report?.counts.attachments, job.report?.placed]).toEqual([
    notes + 1,
    1,
    { parent_id: null, reason: "original" },
  ]);
  await expectBigRestored(db, nervewiki.storageDir, big, {
    notebookId: notebook.id,
    job: job.id,
    byId: adminId,
    client: "api",
  });
  const signed = await getAsset(api, pat, big.photo.id);
  expect(signed.response.status, JSON.stringify(signed.error)).toBe(200);
  expect((await download(nervewiki.baseURL, signed.data?.content_url ?? "")).status).toBe(200);
});

test("TB2 (page): an item deleted from the tree, too large to restore from the trash, is restored in the background; its job's row, reached from the status line, takes the focus and shows its progress while its first batch waits; done, it reports where it put the item, which is in the tree again with its subpages", async ({
  api,
  db,
  nervewiki,
  pageWatch,
  signedInPage,
}, testInfo) => {
  // A job of two batches, held, then waited for: slow runners take longer than a test is given.
  test.slow();
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const index = await createPage(api, pat, notebook.id, "Index");
  const big = await importBig(api, db, pat, notebook.id);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  // Big deleted from the tree as another page shows: shown, its own panels would be read again as the backlinks of
  // its hundred notes change (a links event naming no page, past MaxEventPages), and answer 404 (P4B fix check F-1).
  await page.goto(wikiPagePath(workspace.slug, notebook.id, index.id));
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Index", "Big"]);

  await choosePageAction(page, "Plans", "Big", "Delete");
  const deletion = page.getByRole("alertdialog", { name: "Delete Big?", exact: true });
  await expect(deletion).toContainText(
    `Its subpages go with it: ${String(notes)}. The attachments under it go with it: 1.`
  );
  const deleted = answerTo(page, "DELETE", `/api/v0/nodes/${big.id}`);
  await deletion.getByRole("button", { name: "Delete", exact: true }).click();
  expect((await deleted).status()).toBe(204);
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Index"]);
  await expect(pageHeading(page, "Index")).toBeVisible();
  await expectTrashRoot(db, big.id, { pages: notes + 1, assets: 1, height: 2 }, { alone: true });

  // Too large to restore here: the trash says so, and offers to restore it in the background.
  await page
    .getByRole("navigation", { name: "More of Plans", exact: true })
    .getByRole("link", { name: "Trash", exact: true })
    .click();
  await expect(trashListed(page)).toHaveCount(1);
  const from = "From: the notebook's top level";
  const refused = await restoreWith(page, big.id, "Big", from);
  expect(refused.status()).toBe(422);
  await expect(main.getByRole("alert")).toHaveText(
    `“Big” is too large to restore here: it holds more pages, attachments, content, links or versions than one restore brings back. Pages: ${String(notes + 1)}, attachments: 1.`
  );

  // The notebook's row held, the job's first batch waits: its row shows it running, its progress at none of its
  // nodes, all of them planned.
  const release = await holdNotebookRow(db, notebook.id);
  const row = jobRow(page, "Restore of Big from the trash");
  let job: TransferJob | undefined;
  try {
    const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/trash-restores`);
    await main.getByRole("button", { name: `Restore “Big” in the background (${from})`, exact: true }).click();
    const started = await answer;
    expect(started.status()).toBe(202);
    job = (await started.json()) as TransferJob;
    const status = trashStatus(page);
    await expect(status).toHaveText("Restoring “Big” in the background has started. See its progress");
    await expect(main.getByRole("heading", { level: 1, name: "Trash", exact: true })).toBeFocused();
    await status.getByRole("link", { name: "See its progress", exact: true }).click();
    await expect(row).toBeFocused();
    await expect(row.getByText("Running", { exact: true })).toBeVisible();
    await expect(row.getByRole("progressbar", { name: `Progress: 0 of ${String(nodes)}`, exact: true })).toBeVisible();
  } finally {
    await release();
  }

  // Let go, it ends: its report says what it restored, and where it put the item.
  await expect(row.getByText("Done", { exact: true })).toBeVisible({ timeout: jobTime });
  const report = await openReport(row);
  await expect(report.terms).toHaveText(["Pages", "Attachments", "Renamed", "Skipped, restored by someone first"]);
  await expect(report.counts).toHaveText([String(notes + 1), "1", "0", "0"]);
  await expect(row).toContainText("Put back where it was, at the notebook's top level.");
  await expectBigRestored(db, nervewiki.storageDir, big, {
    notebookId: notebook.id,
    job: job?.id ?? "",
    byId: adminId,
    client: "web",
  });

  // Big is in the tree again, with its notes; its attachment downloads.
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Index", "Big"]);
  await pageTree(page, "Plans").getByRole("link", { name: "Big", exact: true }).click();
  await expect(pageHeading(page, "Big")).toBeVisible();
  await expect.poll(() => subpages(page)).toEqual(Array.from({ length: notes }, (_, at) => note(at + 1)));
  const signed = await getAsset(api, pat, big.photo.id);
  expect(signed.response.status, JSON.stringify(signed.error)).toBe(200);
  expect((await download(nervewiki.baseURL, signed.data?.content_url ?? "")).status).toBe(200);
  pageWatch.expectConsole({ errors: [failedToLoad(422)] });
});
