import { expectAliveSessions } from "../../fixtures/assert/collab";
import { expectSessionGone } from "../../fixtures/assert/page";
import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor } from "../../fixtures/auth";
import { readLock } from "../../fixtures/collab";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, putContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// C6, a lock whose lease ran out (M5 design 4.1, 4.6): an expired session
// holds nothing, through the database rather than the wall clock (v0.1
// design 13.4, item 3).

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
