import { aliveSessionsOf, expectAliveSessions } from "../../fixtures/assert/collab";
import { expectSessionGone, sessionsOf } from "../../fixtures/assert/page";
import { accountIdOf } from "../../fixtures/assert/identity";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { readLock } from "../../fixtures/collab";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, putContent, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { editStatus, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// C6, a lock whose lease ran out (M5 design 4.1, 4.6): an expired session
// holds nothing, through the database rather than the wall clock (v0.1
// design 13.4, item 3). The page versions: released as A is done, as A's
// tab closes, and as A's edit goes 30 minutes without input (M5 design
// 4.7), on the page's clock.

test("C6 (API): once A's session has expired, B opens the page, deleting it; A's save in the expired session is 409 page.edit_session_ended, and the lock names B", async ({
  api,
  db,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const page = await createPage(api, a, notebook.id, "Notes");
  const expired = await openSession(api, a, page.id);
  await db.query(
    "UPDATE edit_sessions SET created_at = now() - interval '3 minutes', expires_at = now() - interval '1 minute' WHERE id = $1",
    [expired.id]
  );

  const opened = await openSession(api, b, page.id);
  await expectAliveSessions(db, page.id, [opened.id]);
  await expectSessionGone(db, expired.id);
  const save = await putContent(api, a, page.id, { content: "Mine\n", base_revision: 1, edit_session_id: expired.id });
  expect([save.response.status, save.error?.code]).toEqual([409, "page.edit_session_ended"]);
  expect((await readLock(api, a, page.id)).holder?.user_id).toBe(await accountIdOf(db, bEmail));
});

test("C6 (page): A done editing, B edits at once; A's tab closed while it edits, its session ends as it goes, and B edits at once, not once the lease is out", async ({
  anotherPage,
  anotherTab,
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminEmail, pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const b = await anotherPage(await joinOnboarded(api, a, workspace.slug, emailFor(testInfo, "b"), "member"));
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const notes = await createPage(api, a, notebook.id, "Notes", null, "Drafted.\n");
  const path = wikiPagePath(workspace.slug, notebook.id, notes.id);
  // The test's page stays blank: A edits in a tab the test can close.
  const tab = await anotherTab(await signedInPage(tokens));
  const aEditing = b.getByText(`${displayNameOf(adminEmail)} is editing this page.`, { exact: true });
  const done = (editor: typeof b) => editor.getByRole("main").getByRole("button", { name: "Done", exact: true });

  await tab.goto(path);
  await startEditing(tab);
  await b.goto(path);
  await expect(aEditing).toBeVisible();
  await done(tab).click();
  await expect(aEditing).toBeHidden();
  await startEditing(b);
  await done(b).click();
  await expect(b.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toBeFocused();

  await startEditing(tab);
  await expect(aEditing).toBeVisible();
  const [held] = await aliveSessionsOf(db, notes.id);
  await tab.close();
  await expect.poll(() => sessionsOf(db, notes.id)).not.toContain(held);
  await expect(aEditing).toBeHidden();
  await startEditing(b);
});

test("C6 (page, idle): A's edit of Notes goes 30 minutes without input: what A typed is saved, the edit is left, the reading view saying why, the focus on Edit; its session is gone, and B edits at once", async ({
  anotherPage,
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const b = await anotherPage(await joinOnboarded(api, a, workspace.slug, emailFor(testInfo, "b"), "member"));
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const notes = await createPage(api, a, notebook.id, "Notes", null, "Drafted.\n");
  const path = wikiPagePath(workspace.slug, notebook.id, notes.id);
  const page = await signedInPage(tokens);
  await page.clock.install();

  await page.goto(path);
  await startEditing(page);
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("One");
  await expect(editStatus(page)).toHaveText("Saved.");
  const [held] = await aliveSessionsOf(db, notes.id);

  await page.clock.fastForward("30:00");
  await expect(page.getByRole("main").getByText("Editing ended after 30 minutes without input.")).toBeVisible();
  await expect(page.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toBeFocused();
  await expect(page.getByRole("article")).toContainText("One");
  await expect.poll(() => sessionsOf(db, notes.id)).not.toContain(held);
  expect((await readContent(api, a, notes.id)).content).toBe("Drafted.\nOne");
  await b.goto(path);
  await startEditing(b);
});
