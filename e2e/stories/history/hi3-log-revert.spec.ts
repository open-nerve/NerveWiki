import type { ApiClient } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectRevert, expectVersions, revertsOf } from "../../fixtures/assert/page";
import { displayNameOf, emailFor, login } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { settleEvents } from "../../fixtures/events";
import { getChangeset, listChanges, listChangesets, revertChangeset } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  endSession,
  listNodes,
  moveNode,
  openSession,
  readContent,
  renameNode,
  writeContent,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  changesetPath,
  historyListed,
  historyPath,
  pageHeading,
  revertDialog,
  revertWith,
  startEditing,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// HI3, a notebook's log and a changeset reverted (M8 design 4.7, 4.8,
// section 9; M8/P3 design 4.8): each write is a changeset of its account
// and client, listed newer first, filtered by account, client and page; a
// changeset says what it changed, node by node. Reverted, what it changed
// goes back as it was before it, a revert of its own: a content's bytes,
// a name, and the links a rename wrote again, as they were, the index and
// the streams following. A changeset whose nodes changed since is refused
// with them, and nothing is written.

/** Alpha's content: a link to Beta, its line ended as Windows ends it. */
const alphaContent = "See [[Beta]].\r\n";
/** Alpha's content as the rename of Beta to Gamma writes it again. */
const rewritten = "See [[Gamma]].\r\n";
/** What the web writes of Notes. */
const hello = "Hello.\n";

/** The ids of what written made: the notebook, its pages, and the changesets of its log, newest first. */
interface Written {
  notebook: string;
  beta: string;
  alpha: string;
  notes: string;
  write: string;
  move: string;
  rename: string;
  createNotes: string;
  createAlpha: string;
  createBeta: string;
}

/**
 * Writes Plans, the notebook notebookId, as the stories read its log: the admin (pat) creates Beta, Alpha linking to
 * it and Notes; B (bPat) renames Beta to Gamma, which writes Alpha's link again; the admin moves Notes under Alpha,
 * and writes Notes through a session of the web (signed in as adminEmail). Each is a changeset of its own.
 */
async function written(
  api: ApiClient,
  pat: string,
  bPat: string,
  adminEmail: string,
  notebookId: string
): Promise<Written> {
  const beta = await createPage(api, pat, notebookId, "Beta");
  const alpha = await createPage(api, pat, notebookId, "Alpha", null, alphaContent);
  const notes = await createPage(api, pat, notebookId, "Notes");
  const renamed = await renameNode(api, bPat, beta.id, "Gamma");
  expect(renamed.response.status, JSON.stringify(renamed.error)).toBe(200);
  expect((await readContent(api, pat, alpha.id)).content).toBe(rewritten);
  await moveNode(api, pat, notes.id, { parent_id: alpha.id });
  const web = await login(api, adminEmail);
  await writeContent(api, web.access_token, notes.id, { content: hello, base_revision: 1 });
  const log = await listChangesets(api, pat, notebookId);
  const [write, move, rename, createNotes, createAlpha, createBeta] = (log.data?.data ?? []).map((each) => each.id);
  return {
    notebook: notebookId,
    beta: beta.id,
    alpha: alpha.id,
    notes: notes.id,
    write: write ?? "",
    move: move ?? "",
    rename: rename ?? "",
    createNotes: createNotes ?? "",
    createAlpha: createAlpha ?? "",
    createBeta: createBeta ?? "",
  };
}

/** Who wrote what of a rename reverted: the admin, who created Alpha, the renamer, and the revert, by whom and how. */
interface RenameReverted {
  adminId: string;
  renamerId: string;
  revertId: string;
  reverterId: string;
  client: string;
}

/**
 * The rename of written reverted, as by says: Beta is Beta again, Alpha's content its bytes again, a version of the
 * revert of its own, and its link leads to Beta in the index.
 */
async function expectRenameReverted(
  api: ApiClient,
  db: Database,
  pat: string,
  of: Written,
  by: RenameReverted
): Promise<void> {
  expect((await listNodes(api, pat, of.notebook)).find((node) => node.id === of.beta)?.name).toBe("Beta");
  expect(await readContent(api, pat, of.alpha)).toMatchObject({ content: alphaContent, revision: 3 });
  await expectVersions(db, of.alpha, [
    { base: null, revision: 1, content: alphaContent, kind: "edit", byId: by.adminId, client: "api" },
    { base: 1, revision: 2, content: rewritten, kind: "edit", byId: by.renamerId, client: "api" },
    {
      base: 2,
      revision: 3,
      content: alphaContent,
      kind: "revert",
      byId: by.reverterId,
      client: by.client,
      revertsId: of.rename,
    },
  ]);
  await expectIndexedLinks(db, of.alpha, [{ kind: "wikilink", property: null, target: "Beta", resolved: of.beta }]);
  await expectRevert(db, by.revertId, by.reverterId, by.client, { revertsId: of.rename, items: 1, versions: 1 });
}

test("HI3 (API): each write is a changeset of its account and client, listed newer first, filtered by account, client and page, a page at a time; a changeset says what it changed; reverted, a content's bytes go back, and a rename's, with the links it wrote, the index and the streams following; one whose page changed since is refused with it, and nothing is written", async ({
  api,
  db,
  nervewiki,
  openEvents,
}, testInfo) => {
  const { adminEmail, adminId, pat, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, pat, workspace.slug, bEmail, "member");
  const bId = await accountIdOf(db, bEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const other = await createNotebook(api, pat, workspace.slug, "Other");
  const elsewhere = await createPage(api, pat, other.id, "Elsewhere");
  const of = await written(api, pat, b, adminEmail, notebook.id);

  const all = await listChangesets(api, pat, notebook.id);
  expect(all.response.status, JSON.stringify(all.error)).toBe(200);
  expect(all.data?.next_cursor).toBeNull();
  expect(
    all.data?.data.map((each) => [
      each.id,
      each.kind,
      each.client,
      each.created_by.user_id,
      each.node_count,
      each.content_count,
    ])
  ).toEqual([
    [of.write, "edit", "web", adminId, 0, 1],
    [of.move, "edit", "api", adminId, 1, 0],
    [of.rename, "edit", "api", bId, 1, 1],
    [of.createNotes, "edit", "api", adminId, 1, 1],
    [of.createAlpha, "edit", "api", adminId, 1, 1],
    [of.createBeta, "edit", "api", adminId, 1, 1],
  ]);
  const rename = all.data?.data[2];
  expect(rename).toEqual({
    id: of.rename,
    notebook_id: notebook.id,
    kind: "edit",
    client: "api",
    message: null,
    created_by: { user_id: bId, display_name: displayNameOf(bEmail) },
    created_at: expect.any(String),
    updated_at: expect.any(String),
    reverts: null,
    reverted_by: [],
    trimmed: false,
    node_count: 1,
    content_count: 1,
    changes: [
      {
        node: { id: of.beta, kind: "page", name: "Gamma", alive: true },
        kind: "renamed",
        before: { parent_id: null, parent_name: null, name: "Beta" },
        after: { parent_id: null, parent_name: null, name: "Gamma" },
        content: null,
      },
      {
        node: { id: of.alpha, kind: "page", name: "Alpha", alive: true },
        kind: "content",
        before: null,
        after: null,
        content: { base_revision: 1, revision: 2 },
      },
    ],
  });
  const ids = async (query: Parameters<typeof listChangesets>[3]) => {
    const listed = await listChangesets(api, pat, notebook.id, query);
    expect(listed.response.status, JSON.stringify(listed.error)).toBe(200);
    return listed.data?.data.map((each) => each.id);
  };
  expect(await ids({ actor: bId })).toEqual([of.rename]);
  expect(await ids({ client: "web" })).toEqual([of.write]);
  expect(await ids({ client: "api", actor: adminId })).toEqual([
    of.move,
    of.createNotes,
    of.createAlpha,
    of.createBeta,
  ]);
  expect(await ids({ pageId: of.alpha })).toEqual([of.rename, of.createAlpha]);
  expect(await ids({ pageId: of.notes, client: "api" })).toEqual([of.move, of.createNotes]);
  const elsewhere404 = await listChangesets(api, pat, notebook.id, { pageId: elsewhere.id });
  expect([elsewhere404.response.status, elsewhere404.error?.code]).toEqual([404, "page.not_found"]);
  // A page at a time, the next after the cursor.
  const first = await listChangesets(api, pat, notebook.id, { limit: 4 });
  expect(first.data?.data.map((each) => each.id)).toEqual([of.write, of.move, of.rename, of.createNotes]);
  const next = await listChangesets(api, pat, notebook.id, { limit: 4, cursor: first.data?.next_cursor ?? "" });
  expect([next.data?.data.map((each) => each.id), next.data?.next_cursor]).toEqual([
    [of.createAlpha, of.createBeta],
    null,
  ]);
  const detail = await getChangeset(api, pat, of.rename);
  expect(detail.data).toEqual({ ...rename, renames: true, moves: false });
  expect((await getChangeset(api, pat, of.move)).data).toMatchObject({ renames: false, moves: true });
  expect((await listChanges(api, pat, of.rename)).data).toEqual({ data: rename?.changes, next_cursor: null });

  // The web's write reverted, with a message: Notes' content is what it was.
  const undoWrite = await revertChangeset(api, pat, of.write, "Not yet");
  expect(undoWrite.response.status, JSON.stringify(undoWrite.error)).toBe(200);
  expect(undoWrite.data?.changed).toBe(true);
  const undoWriteId = undoWrite.data?.changeset_id ?? "";
  expect(await readContent(api, pat, of.notes)).toMatchObject({ content: "", revision: 3 });
  await expectVersions(db, of.notes, [
    { base: null, revision: 1, content: "", kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 2, content: hello, kind: "edit", byId: adminId, client: "web" },
    { base: 2, revision: 3, content: "", kind: "revert", byId: adminId, client: "api", revertsId: of.write },
  ]);
  await expectRevert(db, undoWriteId, adminId, "api", { revertsId: of.write, items: 0, versions: 1 });
  expect((await getChangeset(api, pat, undoWriteId)).data).toMatchObject({
    kind: "revert",
    message: "Not yet",
    reverts: { id: of.write, kind: "edit", created_by: { user_id: adminId } },
  });
  expect((await getChangeset(api, pat, of.write)).data?.reverted_by).toEqual([undoWriteId]);

  // B's rename reverted: Beta is Beta again, and Alpha's bytes are what they were, the streams told.
  await settleEvents(api, nervewiki.baseURL, pat, elsewhere.id);
  const events = await openEvents(pat);
  const undoRename = await revertChangeset(api, pat, of.rename);
  expect(undoRename.response.status, JSON.stringify(undoRename.error)).toBe(200);
  expect(undoRename.data?.changed).toBe(true);
  const ofPlans = { workspace_id: workspace.id, notebook_id: notebook.id };
  const tree = await events.expectNext("pages");
  expect(tree).toMatchObject({ ...ofPlans, tree: true });
  expect(tree.pages).toContainEqual({ id: of.alpha, revision: 3 });
  expect(await events.expectNext("links")).toEqual({ ...ofPlans, pages: [], targets: [of.beta] });
  await expectRenameReverted(api, db, pat, of, {
    adminId,
    renamerId: bId,
    revertId: undoRename.data?.changeset_id ?? "",
    reverterId: adminId,
    client: "api",
  });

  // Notes written since the undo of the web's write: undone, it is refused with Notes, and nothing is written.
  await writeContent(api, pat, of.notes, { content: "Again.\n", base_revision: 3 });
  const refused = await revertChangeset(api, pat, undoWriteId);
  expect([
    refused.response.status,
    refused.error?.code,
    refused.error?.conflicts,
    refused.error?.conflict_count,
  ]).toEqual([409, "page.revert_conflict", [{ node_id: of.notes, name: "Notes", reason: "changed" }], 1]);
  expect(await revertsOf(db, undoWriteId)).toEqual([]);
  expect(await readContent(api, pat, of.notes)).toMatchObject({ content: "Again.\n", revision: 4 });
});

test("HI3 (page): the history in the left column lists the changesets; Only this person and the page menu's history filter it; a rename's page says what it changed; Undo change reverts it, the title focused, saying so, the bytes, the versions and the index following, and the renamed page's backlinks in another tab; a changeset whose page changed since is refused in the dialog, naming it, and nothing is written; an edit session's content is reverted, Undo keeping the focus while the undo is out and after a refusal the dialog keeps", async ({
  anotherTab,
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminEmail, adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, pat, workspace.slug, bEmail, "member");
  const bId = await accountIdOf(db, bEmail);
  const bName = displayNameOf(bEmail);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans", "editor");
  const of = await written(api, pat, b, adminEmail, notebook.id);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  await page.goto(wikiPagePath(workspace.slug, notebook.id, of.alpha));
  await expect(pageHeading(page, "Alpha")).toBeVisible();

  await page
    .getByRole("navigation", { name: "More of Plans", exact: true })
    .getByRole("link", { name: "History", exact: true })
    .click();
  await expect(main.getByRole("heading", { level: 1, name: "History", exact: true })).toBeVisible();
  await expect(page).toHaveTitle("History · Plans · Nerve Wiki");
  const listed = historyListed(page);
  await expect(listed).toHaveCount(6);
  await expect(listed.nth(0)).toContainText(`Edit · ${displayNameOf(adminEmail)}`);
  await expect(listed.nth(0)).toContainText("Web · ");
  await expect(listed.nth(0)).toContainText("Wrote “Notes”");
  await expect(listed.nth(2)).toContainText("Renamed “Beta” to “Gamma”, wrote “Alpha”");

  await listed
    .nth(2)
    .getByRole("button", { name: `Only this person (${bName})`, exact: true })
    .click();
  await expect(listed).toHaveCount(1);
  expect(new URL(page.url()).search).toBe(`?actor=${bId}`);
  await expect(main.getByRole("status")).toHaveText("Listed: 1.");

  await page.goto(wikiPagePath(workspace.slug, notebook.id, of.alpha));
  await main.getByRole("button", { name: "Page actions", exact: true }).click();
  await page.getByRole("menuitem", { name: "This page's history", exact: true }).click();
  await expect(page).toHaveURL(historyPath(workspace.slug, notebook.id, { page: of.alpha }));
  await expect(main.getByText("Page: Alpha", { exact: true })).toBeVisible();
  await expect(listed).toHaveCount(2);

  await listed.first().getByRole("link").first().click();
  await expect(page).toHaveURL(changesetPath(workspace.slug, notebook.id, of.rename));
  const heading = main.getByRole("heading", { level: 1, name: `Edit · ${bName}`, exact: true });
  await expect(heading).toBeVisible();
  const nodes = main.getByRole("list", { name: "Nodes of this change", exact: true }).getByRole("listitem");
  await expect(nodes).toHaveCount(2);
  await expect(nodes.nth(0)).toContainText("Renamed: “Beta” → “Gamma”");
  await expect(
    nodes.nth(1).getByRole("link", { name: "Wrote its content: “Alpha”, version 2", exact: true })
  ).toBeVisible();

  // The renamed page, open in another tab: its backlinks hold Alpha's line as the rename wrote it again.
  const renamedTab = await anotherTab(page);
  await renamedTab.goto(wikiPagePath(workspace.slug, notebook.id, of.beta));
  await expect(pageHeading(renamedTab, "Gamma")).toBeVisible();
  const backlinks = renamedTab
    .getByRole("complementary", { name: "About this page", exact: true })
    .locator("details", { has: renamedTab.getByText("Backlinks", { exact: true }) });
  await expect(backlinks.getByRole("link")).toHaveText(["Alpha"]);
  await expect(backlinks.getByText("See [[Gamma]].", { exact: true })).toBeVisible();

  await main.getByRole("button", { name: "Undo change", exact: true }).click();
  await expect(revertDialog(page)).toContainText(
    "Links written since to the new names or places are not changed back."
  );
  const undone = await revertWith(page, of.rename);
  expect(undone.status()).toBe(200);
  const { changeset_id: revertId } = (await undone.json()) as { changeset_id: string };
  await expect(main.getByRole("status")).toHaveText("Undone. See the undo");
  await expect(heading).toBeFocused();
  await expectRenameReverted(api, db, pat, of, {
    adminId,
    renamerId: bId,
    revertId,
    reverterId: adminId,
    client: "web",
  });
  // The links event the undo sent has the other tab read the backlinks again: Alpha's line as it was (M8 closeout
  // B-M2, C-N12); the pages event, its title.
  await expect(pageHeading(renamedTab, "Beta")).toBeVisible();
  await expect(backlinks.getByText("See [[Beta]].", { exact: true })).toBeVisible();
  await renamedTab.close();

  // Notes written since the web's write: its undo is refused, the dialog naming Notes and why.
  await writeContent(api, pat, of.notes, { content: "Again.\n", base_revision: 2 });
  await page.goto(changesetPath(workspace.slug, notebook.id, of.write));
  const refused = await revertWith(page, of.write);
  expect(refused.status()).toBe(409);
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  const dialog = revertDialog(page);
  await expect(
    dialog.getByRole("heading", { name: "Not undone. Nodes changed since, or that cannot go back: 1", exact: true })
  ).toBeFocused();
  await expect(dialog.getByRole("list", { name: "Nodes in the way", exact: true }).getByRole("listitem")).toHaveText([
    "“Notes” changed since.History of this node",
  ]);
  expect(await revertsOf(db, of.write)).toEqual([]);
  expect(await readContent(api, pat, of.notes)).toMatchObject({ content: "Again.\n", revision: 3 });

  // Notes written through the editor, an edit session: that session's changeset undone (00-M8 section 9).
  await page.goto(wikiPagePath(workspace.slug, notebook.id, of.notes));
  await expect(pageHeading(page, "Notes")).toBeVisible();
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Edited");
  await main.getByRole("button", { name: "Done", exact: true }).click();
  await expect(main.getByRole("button", { name: "Edit", exact: true })).toBeFocused();
  const edited = "Again.\nEdited";
  expect(await readContent(api, pat, of.notes)).toMatchObject({ content: edited, revision: 4 });
  const [session] = (await listChangesets(api, pat, notebook.id)).data?.data ?? [];
  expect(session).toMatchObject({ kind: "edit", client: "web", node_count: 0, content_count: 1 });
  const sessionId = session?.id ?? "";
  await page.goto(historyPath(workspace.slug, notebook.id));
  await expect(listed.nth(0)).toContainText("Wrote “Notes”");
  await listed.nth(0).getByRole("link").first().click();
  await expect(page).toHaveURL(changesetPath(workspace.slug, notebook.id, sessionId));
  const sessionHeading = main.getByRole("heading", { level: 1, name: `Edit · ${displayNameOf(adminEmail)}` });
  await expect(sessionHeading).toBeVisible();

  // B edits Notes: the undo is refused in the dialog, which stays; Undo keeps the focus while the undo is out and
  // after (v0.1 design 13.2, item 12; M8/P3B review A-M1).
  const bSession = await openSession(api, b, of.notes);
  const revertPath = `/api/v0/changesets/${sessionId}/revert`;
  // The undos held, each until released.
  const held: (() => void)[] = [];
  await page.route(`**${revertPath}`, async (route) => {
    await new Promise<void>((resolve) => held.push(resolve));
    await route.continue();
  });
  const releaseHeld = async () => {
    await expect.poll(() => held.length).toBe(1);
    held.shift()?.();
  };
  await main.getByRole("button", { name: "Undo change", exact: true }).click();
  const sessionDialog = revertDialog(page);
  const confirm = sessionDialog.locator("button[type=submit]");
  await confirm.focus();
  const locked = answerTo(page, "POST", revertPath);
  await page.keyboard.press("Enter");
  await expect(confirm).toHaveAttribute("aria-busy", "true");
  await expect(confirm).toHaveText("Undoing…");
  await expect(confirm).toBeFocused();
  await releaseHeld();
  expect((await locked).status()).toBe(409);
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await expect(sessionDialog.getByRole("alert")).toHaveText(`${bName} is editing “Notes”.`);
  await expect(confirm).toHaveText("Undo");
  await expect(confirm).toBeFocused();
  expect(await revertsOf(db, sessionId)).toEqual([]);

  // B done: Enter, pressed twice while the undo is out, sends one undo; the title takes the focus.
  expect((await endSession(api, b, bSession.id)).response.status).toBe(204);
  const reverted = answerTo(page, "POST", revertPath);
  await page.keyboard.press("Enter");
  await expect(confirm).toHaveAttribute("aria-busy", "true");
  await page.keyboard.press("Enter");
  await expect(confirm).toBeFocused();
  await releaseHeld();
  expect((await reverted).status()).toBe(200);
  await expect(main.getByRole("status")).toHaveText("Undone. See the undo");
  await expect(sessionHeading).toBeFocused();
  await page.unroute(`**${revertPath}`);
  expect(pageWatch.apiRequests.filter((each) => each === `POST ${revertPath}`)).toHaveLength(2);
  const sessionReverts = await revertsOf(db, sessionId);
  expect(sessionReverts).toHaveLength(1);
  expect(await readContent(api, pat, of.notes)).toMatchObject({ content: "Again.\n", revision: 5 });
  await expectVersions(db, of.notes, [
    { base: null, revision: 1, content: "", kind: "edit", byId: adminId, client: "api" },
    { base: 1, revision: 2, content: hello, kind: "edit", byId: adminId, client: "web" },
    { base: 2, revision: 3, content: "Again.\n", kind: "edit", byId: adminId, client: "api" },
    { base: 3, revision: 4, content: edited, kind: "edit", byId: adminId, client: "web" },
    { base: 4, revision: 5, content: "Again.\n", kind: "revert", byId: adminId, client: "web", revertsId: sessionId },
  ]);
  await expectRevert(db, sessionReverts[0] ?? "", adminId, "web", { revertsId: sessionId, items: 0, versions: 1 });
});
