import { expectAliveSessions } from "../../fixtures/assert/collab";
import { expectSessionGone, sessionsOf } from "../../fixtures/assert/page";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { failedToLoad } from "../../fixtures/browser";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook, deleteNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  moveNode,
  openSession,
  putContent,
  readContent,
  renameNode,
} from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import {
  choosePageAction,
  editorContent,
  lostBanner,
  pageHeading,
  startEditing,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// C4, what the lock guards (M5 design 4.4): a content write in another
// session than the lock's, and a deletion of what another account holds;
// a rename, a move and a write of the content the page holds pass, and so
// does the holder's own deletion, and a notebook's.

test("C4 (API): while A edits Notes, a write without A's session, B's or A's own token's, is 409 page.locked, as is B's deletion of Notes or of its parent; a rename, a move and a write of the same content pass; A's deletion passes and ends A's session; the notebook's deletion passes", async ({
  api,
  db,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const b = await joinAs(api, a, workspace.slug, emailFor(testInfo, "b"), "member");
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const parent = await createPage(api, a, notebook.id, "Parent");
  const notes = await createPage(api, a, notebook.id, "Notes", parent.id, "Held\n");
  const session = await openSession(api, a, notes.id);

  const writes = [
    await putContent(api, b, notes.id, { content: "Theirs\n", base_revision: 1 }),
    await putContent(api, a, notes.id, { content: "Mine\n", base_revision: 1 }),
  ];
  const deletions = [await deleteNode(api, b, notes.id), await deleteNode(api, b, parent.id)];
  expect([...writes, ...deletions].map((r) => [r.response.status, r.error?.code, r.error?.lock?.page_id])).toEqual([
    [409, "page.locked", notes.id],
    [409, "page.locked", notes.id],
    [409, "page.locked", notes.id],
    [409, "page.locked", notes.id],
  ]);
  expect(await readContent(api, a, notes.id)).toMatchObject({ content: "Held\n", revision: 1 });

  expect((await renameNode(api, b, notes.id, "Renamed")).response.status).toBe(200);
  await moveNode(api, b, notes.id, { parent_id: null });
  const same = await putContent(api, b, notes.id, { content: "Held\n", base_revision: 1 });
  expect([same.response.status, same.data?.revision]).toEqual([200, 1]);
  await expectAliveSessions(db, notes.id, [session.id]);

  expect((await deleteNode(api, a, notes.id)).response.status).toBe(204);
  await expectSessionGone(db, session.id);

  const other = await createPage(api, a, notebook.id, "Other");
  await openSession(api, b, other.id);
  expect((await deleteNotebook(api, a, notebook.id)).response.status).toBe(204);
  expect(await sessionsOf(db, other.id)).toEqual([]);
});

test("C4 (page): while A edits Linux, B's deletion of its parent is refused, the dialog naming A and Linux; A deleting Linux in another tab keeps the editor and its unsaved text, saying the page is gone, until A leaves it", async ({
  anotherPage,
  anotherTab,
  api,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminEmail, pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const b = await anotherPage(await joinOnboarded(api, a, workspace.slug, emailFor(testInfo, "b"), "member"));
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const guide = await createPage(api, a, notebook.id, "Guide");
  const linux = await createPage(api, a, notebook.id, "Linux", guide.id, "Drafted.\n");
  const linuxPath = wikiPagePath(workspace.slug, notebook.id, linux.id);
  const page = await signedInPage(tokens);

  await page.goto(linuxPath);
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Unsaved");

  await b.goto(wikiPagePath(workspace.slug, notebook.id, guide.id));
  await expect(pageHeading(b, "Guide")).toBeVisible();
  await choosePageAction(b, "Plans", "Guide", "Delete");
  const dialog = b.getByRole("alertdialog", { name: "Delete Guide?" });
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog.getByRole("alert")).toHaveText(`${displayNameOf(adminEmail)} is editing “Linux”.`);
  watchOf(b).expectConsole({ errors: [failedToLoad(409)] });
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();

  // A's own edit does not refuse A's deletion (M5 design 4.4).
  const tab = await anotherTab(page);
  await tab.goto(linuxPath);
  await expect(pageHeading(tab, "Linux")).toBeVisible();
  await choosePageAction(tab, "Plans", "Linux", "Delete");
  await tab
    .getByRole("alertdialog", { name: "Delete Linux?" })
    .getByRole("button", { name: "Delete", exact: true })
    .click();
  await expect(pageHeading(tab, "Guide")).toBeVisible();

  await expect(lostBanner(page)).toContainText("This page no longer exists: this editor saves no more.");
  await expect(lostBanner(page)).toContainText("Your changes here are not saved: copy them before you leave.");
  await expect(pageHeading(page, "Linux")).toBeVisible();
  await expect(editorContent(page)).toContainText("Drafted.Unsaved");
  // The session's beat on the event, and the opening it tried again: both not found.
  pageWatch.expectConsole({ errors: [failedToLoad(404), failedToLoad(404)] });
  await lostBanner(page).getByRole("button", { name: "Back to reading", exact: true }).click();
  await page
    .getByRole("alertdialog", { name: "Leave without saving?" })
    .getByRole("button", { name: "Leave", exact: true })
    .click();
  await expect(pageHeading(page, "Page not found")).toBeVisible();
});
