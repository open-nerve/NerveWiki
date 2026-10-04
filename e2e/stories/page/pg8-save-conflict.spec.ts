import { accountIdOf } from "../../fixtures/assert/identity";
import { expectContentWritten, expectOneSessionRevision, sessionsOf } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { countAnswers, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, putContent, readContent, writeContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  conflictRegion,
  contentWrites,
  editorContent,
  editStatus,
  startEditing,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG8, a save that conflicts (M4 design 3, 4): a write that came in first
// refuses a save on the version before it; the editor reads the current
// content, shows the differences, and keeps the user's text on it or
// takes theirs (M4/P6 design 3.8). While the edit session holds the lock,
// no write comes first (M5/P1): it expires, through the database rather
// than the wall clock, another credential writes, and the save in it is
// 409 page.edit_session_ended; the editor opens another, and its save on
// the version before is the conflict.

/** Expires the page pageId's edit sessions, once one is open. */
async function expireSessionsOf(db: Database, pageId: string): Promise<void> {
  await expect.poll(async () => (await sessionsOf(db, pageId)).length).toBe(1);
  await db.query(
    "UPDATE edit_sessions SET created_at = now() - interval '3 minutes', expires_at = now() - interval '1 minute' WHERE node_id = $1",
    [pageId]
  );
}

test("PG8 (API): the session expires and another credential writes first, so a save in it is 409 page.edit_session_ended, and one in a new session on the version before is 409 page.revision_mismatch; the editor reads the current content and keeps theirs on its revision, in the new session's changeset", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editorPat = await joinAs(api, adminPat, workspace.slug, editorEmail, "member");
  const editorId = await accountIdOf(db, editorEmail);
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "editor");
  const page = await createPage(api, adminPat, notebook.id, "Notes");

  const expired = await openSession(api, editorPat, page.id);
  await expireSessionsOf(db, page.id);
  expect((await writeContent(api, adminPat, page.id, { content: "Theirs\n", base_revision: 1 })).revision).toBe(2);
  const ended = await putContent(api, editorPat, page.id, {
    content: "Mine\n",
    base_revision: 1,
    edit_session_id: expired.id,
  });
  expect([ended.response.status, ended.error?.code]).toEqual([409, "page.edit_session_ended"]);
  const session = await openSession(api, editorPat, page.id);
  const stale = await putContent(api, editorPat, page.id, {
    content: "Mine\n",
    base_revision: 1,
    edit_session_id: session.id,
  });
  expect([stale.response.status, stale.error?.code]).toEqual([409, "page.revision_mismatch"]);

  const current = await readContent(api, editorPat, page.id);
  expect(current).toMatchObject({ content: "Theirs\n", revision: 2 });
  const kept = await writeContent(api, editorPat, page.id, {
    content: "Mine\n",
    base_revision: current.revision,
    edit_session_id: session.id,
  });
  expect(kept.revision).toBe(3);
  await expectContentWritten(db, kept, "Mine\n", editorId);
  await expectOneSessionRevision(db, session.id, page.id, 2, 3);
});

test("PG8 (page): the edit's session expires and a token writes; the save opens another session and shows the differences, and Keep mine saves over them on their revision, in the edit's changeset; on another page Discard mine edits theirs, and the next save goes through", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const kept = await createPage(api, pat, notebook.id, "Kept", null, "Base\n");
  const discarded = await createPage(api, pat, notebook.id, "Discarded", null, "Base\n");
  const page = await signedInPage(tokens);

  const viewReads = countAnswers(page, "GET", `/api/v0/pages/${kept.id}/view`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, kept.id));
  // The event stream connects, its refresh reading the page again, before the edit: a connection after the
  // session expired would have the edit beat and open another, which the token's write would find locked.
  await expect.poll(viewReads).toBeGreaterThanOrEqual(2);
  await startEditing(page);
  await expireSessionsOf(db, kept.id);
  await writeContent(api, pat, kept.id, { content: "Base\nTheirs\n", base_revision: 1 });
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Mine");
  const keptWrites = contentWrites(page, kept.id);
  await page.keyboard.press("ControlOrMeta+s");
  const conflict = conflictRegion(page);
  await expect(conflict.getByRole("heading")).toBeFocused();
  await expect(
    conflict.getByRole("textbox", { name: "Your text against the page as it is now", exact: true })
  ).toContainText("Mine");
  await expect(conflict).toContainText("Theirs");
  pageWatch.expectConsole({ errors: [failedToLoad(409), failedToLoad(409)] });
  await conflict.getByRole("button", { name: "Keep mine", exact: true }).click();
  await expect(editStatus(page)).toHaveText("Saved.");
  await expect(conflict).toHaveCount(0);
  expect(await readContent(api, pat, kept.id)).toMatchObject({ content: "Base\nMine", revision: 3 });
  expect(await keptWrites.all()).toEqual([
    { status: 409, code: "page.edit_session_ended" },
    { status: 409, code: "page.revision_mismatch" },
    { status: 200 },
  ]);
  await expectContentWritten(db, await keptWrites.saved(), "Base\nMine", adminId);
  const [keptSession] = await sessionsOf(db, kept.id);
  await expectOneSessionRevision(db, keptSession ?? "", kept.id, 2, 3);

  await page.goto(wikiPagePath(workspace.slug, notebook.id, discarded.id));
  await startEditing(page);
  await expireSessionsOf(db, discarded.id);
  await writeContent(api, pat, discarded.id, { content: "Base\r\nTheirs\r\n", base_revision: 1 });
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("Mine");
  await page.keyboard.press("ControlOrMeta+s");
  pageWatch.expectConsole({ errors: [failedToLoad(409), failedToLoad(409)] });
  await conflict.getByRole("button", { name: "Discard mine", exact: true }).click();
  const content = editorContent(page);
  await expect(content).toBeFocused();
  await expect(content).toContainText("Theirs");
  await expect(content).not.toContainText("Mine");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type("More");
  const discardedWrites = contentWrites(page, discarded.id);
  await page.keyboard.press("ControlOrMeta+s");
  await expect(editStatus(page)).toHaveText("Saved.");
  expect(await readContent(api, pat, discarded.id)).toMatchObject({ content: "Base\r\nTheirs\r\nMore", revision: 3 });
  await expectContentWritten(db, await discardedWrites.saved(), "Base\r\nTheirs\r\nMore", adminId);
  const [discardedSession] = await sessionsOf(db, discarded.id);
  await expectOneSessionRevision(db, discardedSession ?? "", discarded.id, 2, 3);
});
