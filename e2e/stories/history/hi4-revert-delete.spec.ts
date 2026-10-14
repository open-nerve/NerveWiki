import type { ApiClient, Asset } from "@nervewiki/api-client";

import { expectAssetsDeletedWithNodes, expectAssetsRestored } from "../../fixtures/assert/asset";
import {
  expectDeletionRevisions,
  expectRevert,
  expectSubtreeDeleted,
  expectSubtreeRestored,
  revertsOf,
} from "../../fixtures/assert/page";
import { download, getAsset, pngBytes, uploadAsset } from "../../fixtures/assets";
import { displayNameOf } from "../../fixtures/auth";
import type { Database } from "../../fixtures/db";
import { listChangesets, revertChangeset } from "../../fixtures/history";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { trashListed } from "../../fixtures/trash";
import {
  choosePageAction,
  historyListed,
  pageHeading,
  pageTree,
  revertWith,
  treeTitles,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// HI4, a deletion reverted (M8 design 4.8, section 9; M8/P3 design 4.8): a
// subtree deleted with its subpage and attachment, each deletion's item
// holding its page's revision, is back where it was once its deletion is
// reverted, a revert of it, as a restore from the trash puts it back; the
// revert reverted, the subtree is deleted again, back in the trash, the
// two reverts a chain.

/** What a story deletes: Handbook under Guide, Setup under it, and photo.png under Setup. */
interface Subtree {
  guide: string;
  handbook: string;
  setup: string;
  photo: Asset;
}

/** Creates the subtree in the notebook notebookId with pat. */
async function subtree(api: ApiClient, pat: string, notebookId: string): Promise<Subtree> {
  const guide = await createPage(api, pat, notebookId, "Guide");
  const handbook = await createPage(api, pat, notebookId, "Handbook", guide.id, "Start here.\n");
  const setup = await createPage(api, pat, notebookId, "Setup", handbook.id);
  const photo = await uploadAsset(api, pat, notebookId, { name: "photo.png", bytes: pngBytes }, setup.id);
  return { guide: guide.id, handbook: handbook.id, setup: setup.id, photo };
}

/** The newest changeset of the notebook notebookId's log, as pat reads it. */
async function newest(api: ApiClient, pat: string, notebookId: string): Promise<string> {
  const log = await listChangesets(api, pat, notebookId, { limit: 1 });
  expect(log.response.status, JSON.stringify(log.error)).toBe(200);
  return log.data?.data[0]?.id ?? "";
}

/** The deletion of tree, the changeset deletion of deleterId, reverted by the changeset revert from client. */
async function expectDeletionReverted(
  db: Database,
  storageDir: string,
  tree: Subtree,
  { deletion, revert, byId, client }: { deletion: string; revert: string; byId: string; client: string }
): Promise<void> {
  const ids = [tree.handbook, tree.setup, tree.photo.id];
  await expectSubtreeRestored(
    db,
    tree.handbook,
    ids,
    byId,
    { parentId: tree.guide, name: "Handbook" },
    { kind: "revert" }
  );
  await expectAssetsRestored(db, storageDir, [tree.photo]);
  await expectRevert(db, revert, byId, client, { revertsId: deletion, items: 3, versions: 0 });
  expect(await revertsOf(db, deletion)).toEqual([revert]);
}

/** The revert of tree's deletion, the changeset revert, reverted by the changeset again of byId from client. */
async function expectRevertReverted(
  db: Database,
  tree: Subtree,
  {
    deletion,
    revert,
    again,
    byId,
    client,
  }: { deletion: string; revert: string; again: string; byId: string; client: string }
): Promise<void> {
  await expectSubtreeDeleted(db, tree.handbook, [tree.handbook, tree.setup], byId);
  await expectAssetsDeletedWithNodes(db, [tree.photo.id]);
  await expectDeletionRevisions(db, again, [tree.handbook, tree.setup, tree.photo.id]);
  await expectRevert(db, again, byId, client, { revertsId: revert, items: 3, versions: 0 });
  expect([await revertsOf(db, deletion), await revertsOf(db, revert)]).toEqual([[revert], [again]]);
}

test("HI4 (API): a subtree deleted with its subpage and attachment, each item holding its page's revision, is back where it was once its deletion is reverted, its attachment's file there; the revert reverted, it is deleted again", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const tree = await subtree(api, pat, notebook.id);
  expect((await deleteNode(api, pat, tree.handbook)).response.status).toBe(204);
  const deletion = await newest(api, pat, notebook.id);
  await expectDeletionRevisions(db, deletion, [tree.handbook, tree.setup, tree.photo.id]);

  const reverted = await revertChangeset(api, pat, deletion);
  expect(reverted.response.status, JSON.stringify(reverted.error)).toBe(200);
  expect(reverted.data?.changed).toBe(true);
  const revert = reverted.data?.changeset_id ?? "";
  await expectDeletionReverted(db, nervewiki.storageDir, tree, { deletion, revert, byId: adminId, client: "api" });
  const signed = await getAsset(api, pat, tree.photo.id);
  expect(signed.response.status, JSON.stringify(signed.error)).toBe(200);
  expect((await download(nervewiki.baseURL, signed.data?.content_url ?? "")).status).toBe(200);

  const again = await revertChangeset(api, pat, revert);
  expect(again.response.status, JSON.stringify(again.error)).toBe(200);
  expect(again.data?.changed).toBe(true);
  await expectRevertReverted(db, tree, {
    deletion,
    revert,
    again: again.data?.changeset_id ?? "",
    byId: adminId,
    client: "api",
  });
});

test("HI4 (page): a page deleted from the tree, its deletion undone from the history, is in the tree of the same tab again, opens, its attachment downloads; the undo undone, it is in the trash again", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const admin = displayNameOf(adminEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const tree = await subtree(api, pat, notebook.id);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  const places = page.getByRole("navigation", { name: "More of Plans", exact: true });
  await page.goto(wikiPagePath(workspace.slug, notebook.id, tree.handbook));
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "Handbook"]);

  await choosePageAction(page, "Plans", "Handbook", "Delete");
  await page
    .getByRole("alertdialog", { name: "Delete Handbook?", exact: true })
    .getByRole("button", { name: "Delete", exact: true })
    .click();
  await expect(pageHeading(page, "Guide")).toBeFocused();
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide"]);
  const deletion = await newest(api, pat, notebook.id);
  await expectDeletionRevisions(db, deletion, [tree.handbook, tree.setup, tree.photo.id]);

  await places.getByRole("link", { name: "History", exact: true }).click();
  const listed = historyListed(page);
  await expect(listed.first()).toContainText(`Edit · ${admin}`);
  await expect(listed.first()).toContainText("Deleted “Handbook”");
  await listed.first().getByRole("link").first().click();
  await expect(main.getByRole("heading", { level: 1, name: `Edit · ${admin}`, exact: true })).toBeVisible();
  const undone = await revertWith(page, deletion);
  expect(undone.status()).toBe(200);
  const { changeset_id: revert } = (await undone.json()) as { changeset_id: string };
  await expect(main.getByRole("status")).toHaveText("Undone. See the undo");
  await expect(main.getByRole("heading", { level: 1, name: `Edit · ${admin}`, exact: true })).toBeFocused();
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "Handbook"]);
  await expectDeletionReverted(db, nervewiki.storageDir, tree, { deletion, revert, byId: adminId, client: "web" });

  await pageTree(page, "Plans").getByRole("link", { name: "Handbook", exact: true }).click();
  await expect(pageHeading(page, "Handbook")).toBeVisible();
  await expect(main.getByRole("article", { name: "Handbook", exact: true })).toContainText("Start here.");
  const signed = await getAsset(api, pat, tree.photo.id);
  expect(signed.response.status, JSON.stringify(signed.error)).toBe(200);
  expect((await download(nervewiki.baseURL, signed.data?.content_url ?? "")).status).toBe(200);

  // The undo undone, from its own page: the subtree is in the trash again.
  await places.getByRole("link", { name: "History", exact: true }).click();
  await expect(listed.first()).toContainText(`Undo change · ${admin}`);
  await listed.first().getByRole("link").first().click();
  await expect(main.getByRole("heading", { level: 1, name: `Undo change · ${admin}`, exact: true })).toBeVisible();
  const again = await revertWith(page, revert);
  expect(again.status()).toBe(200);
  const { changeset_id: againId } = (await again.json()) as { changeset_id: string };
  await expect(main.getByRole("status")).toHaveText("Undone. See the undo");
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide"]);
  await expectRevertReverted(db, tree, { deletion, revert, again: againId, byId: adminId, client: "web" });
  await places.getByRole("link", { name: "Trash", exact: true }).click();
  await expect(trashListed(page)).toHaveCount(1);
  await expect(trashListed(page).first()).toContainText("Handbook");
});
