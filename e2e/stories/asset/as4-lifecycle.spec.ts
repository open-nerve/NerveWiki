import { existsSync } from "node:fs";

import type { Page } from "@playwright/test";

import {
  blobPath,
  expectAssetsDeletedWithNodes,
  expectAssetsDeletedWithNotebook,
  expectAssetsPurged,
} from "../../fixtures/assert/asset";
import { blobOf, download, getAsset, pngBytes, uploadAsset, utf8, type UploadFile } from "../../fixtures/assets";
import { emailFor } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import { joinAs } from "../../fixtures/invitations";
import { memberOf, removeMember } from "../../fixtures/members";
import { deleteNotebookWith, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import { ownerlessNotebooks } from "../../fixtures/ownerless";
import { listedOwnerless, ownerlessListed, ownerlessPath } from "../../fixtures/ownerless-pages";
import { createPage, deleteNode } from "../../fixtures/pages";
import { deletedDaysAgo } from "../../fixtures/purge";
import { expect, test } from "../../fixtures/test";
import { choosePageAction, pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { createWorkspace, deleteWorkspace, newOnboardedTeam, newTeam, slugFor } from "../../fixtures/workspaces";

// AS4, attachments follow their nodes (M7 design 9; M7/P2 design 3.8): a
// subtree's deletion and a notebook's take their rows at their time; the
// purge, the background job's alone, deletes their files, then their rows,
// before their nodes; a notebook's activity counts its attachments. The
// page version (M7/P4 design 3.5): a page's deletion counts the attachments
// under it and takes them, a notebook's in its settings its own; the
// ownerless list's size and last activity count them. The purge has no
// page: a background job's.

/** A text file named name. */
function text(name: string): UploadFile {
  return { name, bytes: utf8(`The file ${name}.\n`), type: "text/plain" };
}

/** The attachments' section of the page shown, or of the notebook's home. */
const attachments = (page: Page) => page.getByRole("region", { name: "Attachments" });

test("AS4 (API): deleting a subtree or a notebook deletes its attachments' rows at its time; 60 days on, the purge deletes their files and rows before their nodes", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const plans = await createNotebook(api, pat, workspace.slug, "Plans");
  const root = await createPage(api, pat, plans.id, "Root");
  const child = await createPage(api, pat, plans.id, "Child", root.id);
  const deep = await uploadAsset(api, pat, plans.id, { name: "deep.png", bytes: pngBytes }, child.id);
  const shallow = await uploadAsset(api, pat, plans.id, text("shallow.txt"), root.id);
  const kept = await uploadAsset(api, pat, plans.id, text("kept.txt"));
  expect((await deleteNode(api, pat, root.id)).response.status).toBe(204);
  await expectAssetsDeletedWithNodes(db, [deep.id, shallow.id]);
  expect((await getAsset(api, pat, deep.id)).error?.code).toBe("asset.not_found");
  expect((await download(nervewiki.baseURL, deep.content_url)).status).toBe(404);
  expect((await download(nervewiki.baseURL, kept.content_url)).status).toBe(200);

  const drafts = await createNotebook(api, pat, workspace.slug, "Drafts");
  const draft = await uploadAsset(api, pat, drafts.id, text("draft.txt"));
  expect((await deleteNotebook(api, pat, drafts.id)).response.status).toBe(204);
  await expectAssetsDeletedWithNodes(db, [draft.id]);
  await expectAssetsDeletedWithNotebook(db, drafts.id, [draft.id]);

  // The purge: a workspace deleted 61 days ago, its notebook holding a
  // subtree deleted before it and an attachment that was not. One run
  // clears them with no error; a run already going may meet them half
  // moved back and fail, to be retried: the runs that count are those
  // queued after the move.
  const old = await createWorkspace(api, pat, "Old", slugFor(testInfo, "old"));
  const notes = await createNotebook(api, pat, old.slug, "Notes");
  const top = await createPage(api, pat, notes.id, "Top");
  const gone = await uploadAsset(api, pat, notes.id, { name: "gone.png", bytes: pngBytes }, top.id);
  const left = await uploadAsset(api, pat, notes.id, text("left.txt"));
  expect((await deleteNode(api, pat, top.id)).response.status).toBe(204);
  await deleteWorkspace(api, pat, old.slug);
  await deletedDaysAgo(db, old.id, 61);
  const [mark] = await db.query<{ id: string }>("SELECT coalesce(max(id), 0)::text AS id FROM river_job");
  await expect
    .poll(async () => (await db.query("SELECT id FROM workspaces WHERE id = $1", [old.id])).length, {
      message: "the workspace deleted 61 days ago is purged, after its notebook, its pages and its attachments",
      timeout: 15_000,
    })
    .toBe(0);
  await expectAssetsPurged(db, nervewiki.storageDir, [gone, left]);
  const failed = await db.query(
    "SELECT id FROM river_job WHERE kind = 'platform.purge_soft_deleted' AND id > $1::bigint AND errors IS NOT NULL",
    [mark?.id]
  );
  expect(failed, "purge runs that failed").toEqual([]);

  // What was deleted since stays, its rows and its files.
  const younger = [deep, shallow, draft, kept].map(blobOf);
  const rows = await db.query("SELECT id FROM asset_blobs WHERE id = ANY($1::uuid[])", [younger]);
  expect(rows).toHaveLength(4);
  expect(
    younger.filter((blob) => !existsSync(blobPath(nervewiki.storageDir, blob))),
    "files gone"
  ).toEqual([]);
});

test("AS4 (API): an ownerless notebook's size counts its attachments' bytes beside its pages', and its last activity is its latest upload", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  const content = "A page with its content.\n";
  const page = await createPage(api, ownerPat, notebook.id, "Intro", null, content);
  const deleted = await uploadAsset(api, ownerPat, notebook.id, text("deleted.txt"), page.id);
  expect((await deleteNode(api, ownerPat, deleted.id)).response.status).toBe(204);
  const last = await uploadAsset(api, ownerPat, notebook.id, { name: "last.png", bytes: pngBytes }, page.id);
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, workspace.slug, ownerEmail)).id)).response.status
  ).toBe(204);

  const listed = await ownerlessNotebooks(api, adminPat, workspace.slug);
  expect(listed.map((l) => [l.id, l.size_bytes])).toEqual([
    [notebook.id, Buffer.byteLength(content) + pngBytes.length],
  ]);
  const [at] = await db.query<{ last: boolean }>("SELECT $1::timestamptz = $2::timestamptz AS last", [
    listed[0]?.last_activity_at,
    last.created_at,
  ]);
  expect(at?.last, "the last activity is the latest upload").toBe(true);
});

test("AS4 (page): a page's deletion counts the attachments under it and takes them at its time, their addresses no file; the notebook's deletion in its settings takes its own", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const install = await createPage(api, pat, notebook.id, "Install", guide.id);
  const deep = await uploadAsset(api, pat, notebook.id, { name: "deep.png", bytes: pngBytes }, install.id);
  const shallow = await uploadAsset(api, pat, notebook.id, text("shallow.txt"), guide.id);
  const kept = await uploadAsset(api, pat, notebook.id, text("kept.txt"));
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, install.id));
  await expect(pageHeading(page, "Install")).toBeVisible();
  await expect(attachments(page).getByRole("link", { name: "deep.png (opens in a new tab)" })).toBeVisible();

  // Guide, a root page above the page shown: the notebook's home is shown after, its own attachment kept.
  await choosePageAction(page, "Plans", "Guide", "Delete");
  const dialog = page.getByRole("alertdialog", { name: "Delete Guide?", exact: true });
  await expect(dialog).toContainText("Its subpages go with it: 1. The attachments under it go with it: 2.");
  const answer = answerTo(page, "DELETE", `/api/v0/nodes/${guide.id}`);
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  expect((await answer).status()).toBe(204);
  await expect(page).toHaveURL(notebookPath(workspace.slug, notebook.id));
  await expect(attachments(page).getByRole("link", { name: "kept.txt", exact: true })).toBeVisible();
  await expectAssetsDeletedWithNodes(db, [deep.id, shallow.id]);
  for (const gone of [deep, shallow]) {
    // oxlint-disable-next-line no-await-in-loop -- one at a time
    expect((await download(nervewiki.baseURL, gone.content_url)).status, gone.name).toBe(404);
  }
  expect((await download(nervewiki.baseURL, kept.content_url)).status).toBe(200);

  await page.goto(notebookPath(workspace.slug, notebook.id, "general"));
  expect(await deleteNotebookWith(page, notebook.id, "Plans")).toBe(204);
  await expectAssetsDeletedWithNodes(db, [kept.id]);
  await expectAssetsDeletedWithNotebook(db, notebook.id, [kept.id]);
  expect((await download(nervewiki.baseURL, kept.content_url)).status).toBe(404);
});

test("AS4 (page): the ownerless list gives the notebook's size with its attachments' bytes, and its last activity as the day of its latest upload", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const ownerEmail = emailFor(testInfo, "owner");
  const ownerPat = await joinAs(api, adminPat, workspace.slug, ownerEmail, "member");
  const notebook = await createNotebook(api, ownerPat, workspace.slug, "Plans");
  const content = "A page with its content.\n";
  const intro = await createPage(api, ownerPat, notebook.id, "Intro", null, content);
  const last = await uploadAsset(api, ownerPat, notebook.id, { name: "last.png", bytes: pngBytes }, intro.id);
  expect(
    (await removeMember(api, adminPat, (await memberOf(api, adminPat, workspace.slug, ownerEmail)).id)).response.status
  ).toBe(204);
  // The notebook's own change and Intro's write each an earlier day: the upload is the latest.
  await db.query("UPDATE notebooks SET created_at = $2, updated_at = $2 WHERE id = $1", [
    notebook.id,
    "2025-03-01T12:00:00Z",
  ]);
  await db.query(
    `UPDATE changesets SET created_at = $2, updated_at = $2
      WHERE id IN (SELECT changeset_id FROM changeset_items WHERE node_id = $1)`,
    [intro.id, "2025-04-01T12:00:00Z"]
  );
  const page = await signedInPage(tokens);

  await page.goto(ownerlessPath(workspace.slug));

  await expect.poll(() => ownerlessListed(page)).toEqual([listedOwnerless("Plans", ownerEmail, "Private", 0)]);
  const size = Buffer.byteLength(content) + pngBytes.length;
  const latest = await page.evaluate(
    (iso) => new Intl.DateTimeFormat("en", { dateStyle: "medium" }).format(new Date(iso)),
    last.created_at
  );
  expect((await ownerlessListed(page))[0]?.[2]).toMatch(new RegExp(` · Last activity ${latest} · Size ${size} B$`));
});
