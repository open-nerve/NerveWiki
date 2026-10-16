import { expectAssetsDeletedWithNodes, expectAssetsRestored } from "../../fixtures/assert/asset";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectSubtreeRestored, expectTrashRoot } from "../../fixtures/assert/page";
import { download, getAsset, pngBytes, uploadAsset } from "../../fixtures/assets";
import { displayNameOf } from "../../fixtures/auth";
import { answerTo, countAnswers, failedToLoad } from "../../fixtures/browser";
import { holdStream, settleEvents } from "../../fixtures/events";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode, getVersionView, listNodes, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  getTrashItem,
  listTrash,
  restoreNode,
  restoreWith,
  trashedAgo,
  trashListed,
  trashPath,
  trashStatus,
} from "../../fixtures/trash";
import { choosePageAction, pageHeading, pageTree, treeTitles, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// TB1, the trash (M8 design 4.5, 4.6, section 9; M8/P2 design 4.7): a
// subtree deleted with its attachment is one item of the trash, its root
// marked with its counts; listed, its page's view read from where it was,
// its links leading to the pages alive; restored, it is back where it was,
// with its contents, versions, items, attachments' rows and files, and its
// links indexed again, the streams told. An item whose parent is in the
// trash goes back at the top level; one whose name a sibling took since is
// numbered; one past its time, before the purge takes it, is listed no
// more and refused.

test("TB1 (API): a subtree deleted with an attachment is one trash item of its counts, its page's view read from where it was; restored, it is back where it was with its contents, versions, items, rows, files and links, the streams told; a page whose parent is in the trash goes back at the top level, one whose name was taken numbered, one past its time not at all", async ({
  api,
  db,
  nervewiki,
  openEvents,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const marker = await createPage(api, pat, notebook.id, "Marker");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const elsewhere = await createPage(api, pat, notebook.id, "Elsewhere");
  const handbook = await createPage(api, pat, notebook.id, "Handbook", guide.id, "Start with [[Elsewhere]].\n");
  const setup = await createPage(api, pat, notebook.id, "Setup", handbook.id);
  const photo = await uploadAsset(api, pat, notebook.id, { name: "photo.png", bytes: pngBytes }, setup.id);
  const setupContent = "See [[Elsewhere]] and ![[photo.png]].\n";
  await writeContent(api, pat, setup.id, { content: setupContent, base_revision: 1 });
  const linker = await createPage(api, pat, notebook.id, "Linker", null, "Read [[Setup]].\n");
  const setupLinks = [
    { kind: "wikilink" as const, property: null, target: "Elsewhere", resolved: elsewhere.id },
    { kind: "embed" as const, property: null, target: "photo.png", resolved: photo.id },
  ];
  await expectIndexedLinks(db, setup.id, setupLinks);

  expect((await deleteNode(api, pat, handbook.id)).response.status).toBe(204);
  await expectTrashRoot(db, handbook.id, { pages: 2, assets: 1, height: 2 }, { alone: true });
  await expectAssetsDeletedWithNodes(db, [photo.id]);
  const listed = await listTrash(api, pat, notebook.id);
  expect(listed.response.status, JSON.stringify(listed.error)).toBe(200);
  expect(listed.data?.next_cursor).toBeNull();
  expect(listed.data?.data).toEqual([
    {
      id: handbook.id,
      notebook_id: notebook.id,
      kind: "page",
      name: "Handbook",
      ancestors: [{ id: guide.id, name: "Guide" }],
      deleted_at: expect.any(String),
      deleted_by: { user_id: adminId, display_name: displayNameOf(adminEmail) },
      restorable_until: expect.any(String),
      pages: 2,
      assets: 1,
      restore_to: { parent_id: guide.id, reason: "original" },
      revision: 1,
      asset: null,
    },
  ]);
  const item = listed.data?.data[0];
  // Restorable for the purge's retention, 60 days, less the hour before the purge.
  expect(Date.parse(item?.restorable_until ?? "") - Date.parse(item?.deleted_at ?? "")).toBe(5_180_400_000);
  expect((await getTrashItem(api, pat, handbook.id)).data).toEqual(item);
  const member = await getTrashItem(api, pat, setup.id);
  expect([member.response.status, member.error?.code, member.error?.root_id]).toEqual([
    409,
    "page.not_trash_root",
    handbook.id,
  ]);
  // The item's page as it was deleted, its link leading to the page alive.
  const view = await getVersionView(api, pat, handbook.id, item?.revision ?? 0);
  expect(view.response.status, JSON.stringify(view.error)).toBe(200);
  expect(view.data?.html).toContain(`data-nw-node="${elsewhere.id}"`);

  await settleEvents(api, nervewiki.baseURL, pat, marker.id);
  const events = await openEvents(pat);
  const restored = await restoreNode(api, pat, handbook.id);
  expect(restored.response.status, JSON.stringify(restored.error)).toBe(200);
  expect(restored.data).toEqual({
    id: handbook.id,
    kind: "page",
    name: "Handbook",
    renamed: false,
    placed: { parent_id: guide.id, reason: "original" },
    changeset_id: expect.stringMatching(/^[0-9a-f-]{36}$/),
  });
  const ofPlans = { workspace_id: workspace.id, notebook_id: notebook.id };
  const tree = await events.expectNext("pages");
  expect({ ...tree, pages: tree.pages?.toSorted((a, b) => a.id.localeCompare(b.id)) }).toEqual({
    ...ofPlans,
    tree: true,
    pages: [
      { id: handbook.id, revision: 1 },
      { id: setup.id, revision: 2 },
    ].toSorted((a, b) => a.id.localeCompare(b.id)),
  });
  const links = await events.expectNext("links");
  // Linker's link resolves again; the restored pages' links, and Linker's, are backlinks of their targets again.
  expect({ ...links, targets: links.targets?.toSorted() }).toEqual({
    ...ofPlans,
    pages: [linker.id],
    targets: [elsewhere.id, photo.id, setup.id].toSorted(),
    aliases: false,
  });
  await expectSubtreeRestored(
    db,
    handbook.id,
    [handbook.id, setup.id, photo.id],
    adminId,
    { parentId: guide.id, name: "Handbook" },
    { kind: "trash_restore" }
  );
  await expectAssetsRestored(db, nervewiki.storageDir, [photo]);
  await expectIndexedLinks(db, setup.id, setupLinks);
  await expectIndexedLinks(db, linker.id, [{ kind: "wikilink", property: null, target: "Setup", resolved: setup.id }]);
  const signed = await getAsset(api, pat, photo.id);
  expect(signed.response.status, JSON.stringify(signed.error)).toBe(200);
  expect((await download(nervewiki.baseURL, signed.data?.content_url ?? "")).status).toBe(200);
  expect((await listTrash(api, pat, notebook.id)).data?.data).toEqual([]);

  // A page deleted, then its parent: restored, the page goes back at the top level, its parent being in the trash.
  const old = await createPage(api, pat, notebook.id, "Old");
  const older = await createPage(api, pat, notebook.id, "Older", old.id);
  expect((await deleteNode(api, pat, older.id)).response.status).toBe(204);
  expect((await deleteNode(api, pat, old.id)).response.status).toBe(204);
  await expectTrashRoot(db, older.id, { pages: 1, assets: 0, height: 1 }, { alone: true });
  await expectTrashRoot(db, old.id, { pages: 1, assets: 0, height: 1 }, { alone: true });
  expect((await getTrashItem(api, pat, older.id)).data?.restore_to).toEqual({
    parent_id: null,
    reason: "parent_in_trash",
  });
  const top = await restoreNode(api, pat, older.id);
  expect([top.response.status, top.data?.placed, top.data?.renamed]).toEqual([
    200,
    { parent_id: null, reason: "parent_in_trash" },
    false,
  ]);
  await expectSubtreeRestored(
    db,
    older.id,
    [older.id],
    adminId,
    { parentId: null, name: "Older" },
    { kind: "trash_restore" }
  );

  // A page whose name a page made where it was took since: restored, it is numbered.
  const notes = await createPage(api, pat, notebook.id, "Notes", guide.id);
  expect((await deleteNode(api, pat, notes.id)).response.status).toBe(204);
  await createPage(api, pat, notebook.id, "Notes", guide.id);
  const numbered = await restoreNode(api, pat, notes.id);
  expect([numbered.response.status, numbered.data?.name, numbered.data?.renamed, numbered.data?.placed]).toEqual([
    200,
    "Notes 2",
    true,
    { parent_id: guide.id, reason: "original" },
  ]);
  await expectSubtreeRestored(
    db,
    notes.id,
    [notes.id],
    adminId,
    { parentId: guide.id, name: "Notes 2" },
    { kind: "trash_restore" }
  );
  expect(
    (await listNodes(api, pat, notebook.id)).filter((node) => node.parent_id === guide.id).map((node) => node.name)
  ).toEqual(expect.arrayContaining(["Handbook", "Notes", "Notes 2"]));

  // A page deleted the purge's retention less half an hour ago, moved back so from its time a moment ago: past its
  // time, before the purge (every two seconds here) may take it, an hour later.
  const stale = await createPage(api, pat, notebook.id, "Stale");
  await uploadAsset(api, pat, notebook.id, { name: "stale.png", bytes: pngBytes }, stale.id);
  expect((await deleteNode(api, pat, stale.id)).response.status).toBe(204);
  // Its content, version and items, and its attachment's node and row, deleted at its node's time, as the group is.
  const group = () =>
    db.query<{ contents: number; versions: number; items: number; attachments: number; rows: number }>(
      `SELECT (SELECT count(*) FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at = n.deleted_at)::int AS contents,
              (SELECT count(*) FROM page_revisions v WHERE v.node_id = n.id AND v.deleted_at = n.deleted_at)::int AS versions,
              (SELECT count(*) FROM changeset_items i WHERE i.node_id = n.id AND i.deleted_at = n.deleted_at)::int AS items,
              (SELECT count(*) FROM nodes a WHERE a.parent_id = n.id AND a.deleted_at = n.deleted_at)::int AS attachments,
              (SELECT count(*) FROM asset_blobs b JOIN nodes a ON a.id = b.node_id
                WHERE a.parent_id = n.id AND b.deleted_at = n.deleted_at)::int AS rows
         FROM nodes n WHERE n.id = $1`,
      [stale.id]
    );
  const deleted = await group();
  expect(deleted).toEqual([{ contents: 1, versions: 1, items: expect.any(Number), attachments: 1, rows: 1 }]);
  await trashedAgo(db, stale.id, "1439 hours 30 minutes");
  expect(await group(), "the group moved back whole").toEqual(deleted);
  expect((await listTrash(api, pat, notebook.id)).data?.data.map((each) => each.name)).toEqual(["Old"]);
  const expired = await getTrashItem(api, pat, stale.id);
  expect([expired.response.status, expired.error?.code]).toEqual([409, "page.restore_expired"]);
  const refused = await restoreNode(api, pat, stale.id);
  expect([refused.response.status, refused.error?.code]).toEqual([409, "page.restore_expired"]);
  const [kept] = await db.query<{ deleted: boolean }>(
    "SELECT deleted_at IS NOT NULL AS deleted FROM nodes WHERE id = $1",
    [stale.id]
  );
  expect(kept?.deleted, "the item past its time is still in the trash, the purge's to take").toBe(true);
});

test("TB1 (page): a page deleted from the tree, its confirmation saying the trash and its days, is in the notebook's trash; its preview's links lead to the pages alive; restored from the trash, it is in the tree of the same tab again with its links and attachment, and opens; a page whose parent is in the trash is restored at the top level, one whose name was taken numbered, as the trash says", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const elsewhere = await createPage(api, pat, notebook.id, "Elsewhere");
  const handbook = await createPage(api, pat, notebook.id, "Handbook", guide.id, "Start with [[Elsewhere]].\n");
  const setup = await createPage(api, pat, notebook.id, "Setup", handbook.id);
  const photo = await uploadAsset(api, pat, notebook.id, { name: "photo.png", bytes: pngBytes }, setup.id);
  const handbookLinks = [{ kind: "wikilink" as const, property: null, target: "Elsewhere", resolved: elsewhere.id }];
  await expectIndexedLinks(db, handbook.id, handbookLinks);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  await page.goto(wikiPagePath(workspace.slug, notebook.id, handbook.id));
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "Handbook", "Elsewhere"]);

  await choosePageAction(page, "Plans", "Handbook", "Delete");
  const dialog = page.getByRole("alertdialog", { name: "Delete Handbook?", exact: true });
  await expect(dialog).toContainText(
    "It moves to the trash, where it can be restored for 59 days; whoever can read this notebook still sees it there. Its subpages go with it: 1. The attachments under it go with it: 1."
  );
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  // The page shown gone, its shell goes to its parent.
  await expect(pageHeading(page, "Guide")).toBeFocused();
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "Elsewhere"]);
  await expectTrashRoot(db, handbook.id, { pages: 2, assets: 1, height: 2 }, { alone: true });
  await expectAssetsDeletedWithNodes(db, [photo.id]);

  const toTrash = page
    .getByRole("navigation", { name: "More of Plans", exact: true })
    .getByRole("link", { name: "Trash", exact: true });
  await toTrash.click();
  await expect(main.getByRole("heading", { level: 1, name: "Trash", exact: true })).toBeVisible();
  await expect(page).toHaveTitle("Trash · Plans · Nerve Wiki");
  const listed = trashListed(page);
  await expect(listed).toHaveCount(1);
  await expect(listed.first()).toContainText("From: Guide");
  await expect(listed.first()).toContainText("Pages: 2, attachments: 1");
  await expect(listed.first()).toContainText("Restores under “Guide”");

  await listed.first().getByRole("link", { name: "Handbook (From: Guide)", exact: true }).click();
  await expect(page).toHaveURL(trashPath(workspace.slug, notebook.id, handbook.id));
  const article = main.getByRole("article", { name: "Handbook", exact: true });
  await expect(article).toContainText("Start with Elsewhere.");
  await article.getByRole("link", { name: "Elsewhere", exact: true }).click();
  await expect(pageHeading(page, "Elsewhere")).toBeVisible();
  await page.goBack();
  await expect(article).toBeVisible();
  await main.getByRole("navigation", { name: "Breadcrumb", exact: true }).getByRole("link", { name: "Trash" }).click();

  const answer = await restoreWith(page, handbook.id, "Handbook", "From: Guide");
  expect(answer.status()).toBe(200);
  const status = trashStatus(page);
  await expect(status).toHaveText("Restored “Handbook” under “Guide”. Open");
  await expect(main.getByRole("heading", { level: 1, name: "Trash", exact: true })).toBeFocused();
  await expect(main.getByText("The trash is empty.", { exact: true })).toBeVisible();
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "Handbook", "Elsewhere"]);
  await expectSubtreeRestored(
    db,
    handbook.id,
    [handbook.id, setup.id, photo.id],
    adminId,
    { parentId: guide.id, name: "Handbook" },
    { kind: "trash_restore" }
  );
  await expectAssetsRestored(db, nervewiki.storageDir, [photo]);
  await expectIndexedLinks(db, handbook.id, handbookLinks);
  const signed = await getAsset(api, pat, photo.id);
  expect(signed.response.status, JSON.stringify(signed.error)).toBe(200);
  expect((await download(nervewiki.baseURL, signed.data?.content_url ?? "")).status).toBe(200);

  await status.getByRole("link", { name: "Open", exact: true }).click();
  await expect(pageHeading(page, "Handbook")).toBeFocused();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, handbook.id));
  await expect(main.getByRole("article", { name: "Handbook", exact: true }).getByRole("link")).toHaveText("Elsewhere");

  // A page deleted, then its parent; a page deleted whose name a page made where it was took since (M8 closeout
  // B-M2, C-M4): the trash says where each goes back, and restored from it, says where each went.
  const old = await createPage(api, pat, notebook.id, "Old");
  const older = await createPage(api, pat, notebook.id, "Older", old.id);
  expect((await deleteNode(api, pat, older.id)).response.status).toBe(204);
  expect((await deleteNode(api, pat, old.id)).response.status).toBe(204);
  const notes = await createPage(api, pat, notebook.id, "Notes", guide.id);
  expect((await deleteNode(api, pat, notes.id)).response.status).toBe(204);
  await createPage(api, pat, notebook.id, "Notes", guide.id);
  await toTrash.click();
  await expect(listed).toHaveCount(3);
  await expect(listed.filter({ hasText: "Older" })).toContainText(
    "Restores at the top level: the page it was under is in the trash"
  );
  await expect(listed.filter({ hasText: "Notes" })).toContainText("Restores under “Guide”");

  expect((await restoreWith(page, older.id, "Older", "From: Old")).status()).toBe(200);
  await expect(status).toHaveText("Restored “Older” at the notebook's top level. Open");
  await expect(pageTree(page, "Plans").getByRole("link", { name: "Older", exact: true })).toBeVisible();
  await expectSubtreeRestored(
    db,
    older.id,
    [older.id],
    adminId,
    { parentId: null, name: "Older" },
    { kind: "trash_restore" }
  );
  expect((await restoreWith(page, notes.id, "Notes", "From: Guide")).status()).toBe(200);
  await expect(status).toHaveText("Restored as “Notes 2” under “Guide”: a page or attachment there had its name. Open");
  await expectSubtreeRestored(
    db,
    notes.id,
    [notes.id],
    adminId,
    { parentId: guide.id, name: "Notes 2" },
    { kind: "trash_restore" }
  );
  await expect(listed).toHaveCount(1);
  await expect(listed.first()).toContainText("Old");
});

// M8/P2B review A-I1, C-I1: the restore's own event reads the item again as the restore's tree read is out.
test("TB1 (preview): a page restored from its preview, its own event reading it again at once, goes back to the trash, which says where it went, its title taking the focus", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const handbook = await createPage(api, pat, notebook.id, "Handbook", guide.id, "Hello.\n");
  expect((await deleteNode(api, pat, handbook.id)).response.status).toBe(204);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  // The stream let in once the preview shows: its connection reads the item again; the refresher has read nothing.
  const letIn = await holdStream(page);
  await page.goto(trashPath(workspace.slug, notebook.id, handbook.id));
  await expect(main.getByRole("article", { name: "Handbook", exact: true })).toContainText("Hello.");
  const itemPath = `/api/v0/nodes/${handbook.id}/trash-item`;
  const reread = answerTo(page, "GET", itemPath);
  letIn();
  expect((await reread).status()).toBe(200);
  // The item read again after the restore's answer, and by its event, answers it is no longer in the trash.
  const rereads = countAnswers(page, "GET", itemPath);
  // The item shown stays while the restore is out: nothing says it is gone (M8/P2B review A-I1).
  await page.evaluate((text) => {
    const said = { gone: false };
    Object.assign(window, { nervewikiE2eGone: said });
    new MutationObserver(() => {
      said.gone ||= document.body.textContent.includes(text);
    }).observe(document.body, { childList: true, subtree: true, characterData: true });
  }, "This is no longer in the trash.");

  const answer = await restoreWith(page, handbook.id, "Handbook", "From: Guide");
  expect(answer.status()).toBe(200);
  await expect(page).toHaveURL(trashPath(workspace.slug, notebook.id));
  await expect(trashStatus(page)).toHaveText("Restored “Handbook” under “Guide”. Open");
  await expect(main.getByRole("heading", { level: 1, name: "Trash", exact: true })).toBeFocused();
  await expect(main.getByText("The trash is empty.", { exact: true })).toBeVisible();
  await expectSubtreeRestored(
    db,
    handbook.id,
    [handbook.id],
    adminId,
    { parentId: guide.id, name: "Handbook" },
    { kind: "trash_restore" }
  );
  expect(
    await page.evaluate(() => (window as unknown as { nervewikiE2eGone: { gone: boolean } }).nervewikiE2eGone.gone)
  ).toBe(false);
  expect(rereads()).toBeGreaterThanOrEqual(1);
  pageWatch.expectConsole({ errors: Array.from({ length: rereads() }, () => failedToLoad(409)) });
});
