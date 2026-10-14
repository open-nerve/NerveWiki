import type { TransferJob } from "@nervewiki/api-client";

import { expectRevert, expectTrashRoot, revertsOf } from "../../fixtures/assert/page";
import { expectBackgroundJob } from "../../fixtures/assert/transfer";
import { displayNameOf } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { revertChangeset } from "../../fixtures/history";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, startImport, startRevert } from "../../fixtures/transfer";
import { trashListed } from "../../fixtures/trash";
import { changesetPath, revertDialog, revertWith, treeTitles } from "../../fixtures/wiki-pages";
import { importWith, jobRow, openReport } from "../../fixtures/wiki-transfer";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// HI5, an import undone in the background (M8 design 4.8, 4.10, section 9;
// M8/P4 design 4.5): an import of more pages at the notebook's top than
// one undo takes in a round is too large to undo here; undone in the
// background, a job deletes them batch by batch, each page an item of the
// trash of its own, in one changeset, a revert of the import's, which the
// import's job names.

/** How many empty notes the vault holds at its top: one more than an undo's round takes (MaxRevertSteps). */
const notes = 1_501;

/** How long a job of the story is given: each is a dozen batches and more, longer than a job's default wait. */
const jobTime = 60_000;

/** A vault of notes empty notes at its top, Note 0001 to Note 1501. */
function vault(): Buffer {
  return zipOf(
    Array.from({ length: notes }, (_, at) => ({ name: `Note ${String(at + 1).padStart(4, "0")}.md`, data: "" }))
  );
}

/**
 * transfer_jobs, changesets, changeset_items, nodes: the import importJob wrote the changeset imported, its row
 * naming it, of the notes; the job revertJob of byId from client reverted it in the background, its changeset a revert
 * of it, the only one, of an item of each note, which is a trash item of its own (read from the database, not from a
 * page: the history counts to 1,000).
 */
async function expectImportReverted(
  db: Database,
  {
    importJob,
    imported,
    revertJob,
    byId,
    client,
  }: {
    importJob: string;
    imported: string;
    revertJob: string;
    byId: string;
    client: string;
  }
): Promise<void> {
  expect(
    await db.query(
      `SELECT j.changeset_id, s.kind, (SELECT count(*)::int FROM changeset_items i WHERE i.changeset_id = s.id) AS items
         FROM transfer_jobs j JOIN changesets s ON s.id = j.changeset_id WHERE j.id = $1`,
      [importJob]
    )
  ).toEqual([{ changeset_id: imported, kind: "import", items: notes }]);
  const revert = await expectBackgroundJob(db, revertJob, {
    kind: "revert",
    creatorId: byId,
    client,
    root: null,
    target: imported,
    nodes: notes,
    counts: { deleted: notes },
    placed: null,
  });
  await expectRevert(db, revert, byId, client, { revertsId: imported, items: notes, versions: 0 });
  expect(await revertsOf(db, imported)).toEqual([revert]);
  // Each of its items a note the import made.
  const deleted = await db.query<{ node_id: string }>(
    `SELECT r.node_id FROM changeset_items r JOIN changeset_items i ON i.node_id = r.node_id AND i.changeset_id = $2
      WHERE r.changeset_id = $1`,
    [revert, imported]
  );
  expect(deleted).toHaveLength(notes);
  await Promise.all(deleted.map(({ node_id: id }) => expectTrashRoot(db, id, { pages: 1, assets: 0, height: 1 })));
}

test("HI5 (API): an import of more notes than an undo takes is refused as too large to undo; undone in the background, each note is a trash item of its own, in a revert of the import's changeset", async ({
  api,
  db,
}, testInfo) => {
  // Two jobs of a dozen batches and more each, waited for: slow runners take longer than a test is given.
  test.slow();
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Notes");
  const imported = await endedJob(api, pat, (await startImport(api, pat, notebook.id, vault())).id, jobTime);
  expect([imported.state, imported.progress]).toEqual(["succeeded", { done: notes, total: notes }]);
  const changeset = imported.changeset_id ?? "";

  const refused = await revertChangeset(api, pat, changeset);
  expect([refused.response.status, refused.error?.code]).toEqual([422, "page.revert_too_large"]);
  const queued = await startRevert(api, pat, notebook.id, changeset);
  expect([queued.kind, queued.state, queued.name, queued.root_id, queued.target_changeset_id, queued.client]).toEqual([
    "revert",
    "queued",
    "import",
    null,
    changeset,
    "api",
  ]);
  const job = await endedJob(api, pat, queued.id, jobTime);
  expect([job.state, job.progress, job.changeset_id]).toEqual([
    "succeeded",
    { done: notes, total: notes },
    queued.changeset_id,
  ]);
  expect(job.report?.counts).toEqual({
    pages: 0,
    attachments: 0,
    renamed: 0,
    missing: 0,
    skipped: 0,
    restored: 0,
    deleted: notes,
    put_back: 0,
    written: 0,
  });
  await expectImportReverted(db, {
    importJob: imported.id,
    imported: changeset,
    revertJob: job.id,
    byId: adminId,
    client: "api",
  });
  expect(await listNodes(api, pat, notebook.id)).toEqual([]);
});

test("HI5 (page): an import of more notes than an undo takes, its changeset reached from its job, is undone in the background from the dialog that says it is too large; its job's row, reached from the status line, takes the focus and reports the notes deleted, which the trash lists and the tree no longer shows", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  // Two jobs of a dozen batches and more each, waited for: slow runners take longer than a test is given.
  test.slow();
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const admin = displayNameOf(adminEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Notes");
  await createPage(api, pat, notebook.id, "Index");
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  await page.goto(notebookPath(workspace.slug, notebook.id, "transfer"));
  const queued = await importWith(page, notebook.id, vault(), { name: "notes.zip" });
  const changeset = queued.changeset_id ?? "";
  const imports = jobRow(page, "Import of notes.zip");
  await expect(imports.getByText("Done", { exact: true })).toBeVisible({ timeout: jobTime });

  // The import's changeset, from its row: more nodes than the history counts.
  await imports.getByRole("link", { name: /^See in the history \(Import of notes\.zip, / }).click();
  await expect(page).toHaveURL(changesetPath(workspace.slug, notebook.id, changeset));
  const heading = main.getByRole("heading", { level: 1, name: `Import · ${admin}`, exact: true });
  await expect(heading).toBeVisible();
  await expect(main.getByText(/^Nodes: 1,000\+, contents: /)).toBeVisible();

  // Too large to undo here, the dialog asks to undo it in the background, its title taking the focus.
  const refused = await revertWith(page, changeset);
  expect(refused.status()).toBe(422);
  const dialog = revertDialog(page);
  const tooLarge = dialog.getByRole("heading", { level: 3 });
  await expect(tooLarge).toHaveText(
    "This change is too large to undo here: its nodes, pages, content, links or those pages' history are more than one undo does. Nodes it touched: 1,000+."
  );
  await expect(tooLarge).toBeFocused();
  const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/reverts`);
  await dialog.getByRole("button", { name: "Undo in the background", exact: true }).click();
  const started = await answer;
  expect(started.status()).toBe(202);
  const job = (await started.json()) as TransferJob;
  await expect(dialog).toBeHidden();
  const status = main.getByRole("status");
  await expect(status).toHaveText("Undoing this change in the background has started. See its progress");
  await expect(heading).toBeFocused();

  // Its row, reached from the status line, takes the focus; done, it reports the notes deleted.
  await status.getByRole("link", { name: "See its progress", exact: true }).click();
  await expect(page).toHaveURL(notebookPath(workspace.slug, notebook.id, "transfer"));
  const row = jobRow(page, "Undo: Import");
  await expect(row).toBeFocused();
  await expect(row.getByText("Done", { exact: true })).toBeVisible({ timeout: jobTime });
  const report = await openReport(row);
  await expect(report.terms).toHaveText([
    "Nodes restored",
    "Nodes deleted",
    "Nodes put back",
    "Pages written back",
    "Skipped, restored by someone first",
  ]);
  await expect(report.counts).toHaveText(["0", "1,501", "0", "0", "0"]);
  await expectImportReverted(db, {
    importJob: queued.id,
    imported: changeset,
    revertJob: job.id,
    byId: adminId,
    client: "web",
  });

  // The trash lists the notes, a page of them first; the tree shows the page there before alone.
  await page
    .getByRole("navigation", { name: "More of Notes", exact: true })
    .getByRole("link", { name: "Trash", exact: true })
    .click();
  await expect(trashListed(page)).toHaveCount(50);
  await expect(trashListed(page).first()).toContainText("From: the notebook's top level");
  await expect(main.getByRole("button", { name: "More of the trash", exact: true })).toBeVisible();
  await expect.poll(() => treeTitles(page, "Notes")).toEqual(["Index"]);
  pageWatch.expectConsole({ errors: [failedToLoad(422)] });
});
