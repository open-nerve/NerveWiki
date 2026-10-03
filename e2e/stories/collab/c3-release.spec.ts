import { expectAliveSessions, expectTombstone } from "../../fixtures/assert/collab";
import { accountIdOf } from "../../fixtures/assert/identity";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { readLock, releaseLock } from "../../fixtures/collab";
import { joinAs } from "../../fixtures/invitations";
import { addNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, putContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// C3, the notebook admin's forced unlock (M5 design 4.2): an editor or a
// reader may not; the admin may, and the editor's next save says who.

test("C3 (API): an editor's and a reader's release of A's lock are 403; the notebook's admin's is 204, and A's save is then 409 page.edit_session_unlocked naming the admin; the lock is free", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: admin, workspace } = await newTeam(api, testInfo);
  const aEmail = emailFor(testInfo, "a");
  const a = await joinAs(api, admin, workspace.slug, aEmail, "member");
  const editor = await joinAs(api, admin, workspace.slug, emailFor(testInfo, "editor"), "member");
  const reader = await joinAs(api, admin, workspace.slug, emailFor(testInfo, "reader"), "member");
  const notebook = await createNotebook(api, admin, workspace.slug, "Plans", "viewer");
  const added = await Promise.all(
    [aEmail, emailFor(testInfo, "editor")].map(async (email) =>
      addNotebookMember(api, admin, notebook.id, await accountIdOf(db, email), "editor")
    )
  );
  expect(added.map((r) => r.response.status)).toEqual([201, 201]);
  const page = await createPage(api, admin, notebook.id, "Notes");
  const session = await openSession(api, a, page.id);

  const refused = [await releaseLock(api, editor, page.id), await releaseLock(api, reader, page.id)];
  expect(refused.map((r) => [r.response.status, r.error?.code])).toEqual([
    [403, "forbidden"],
    [403, "forbidden"],
  ]);
  await expectAliveSessions(db, page.id, [session.id]);

  expect((await releaseLock(api, admin, page.id)).response.status).toBe(204);
  await expectTombstone(db, session.id, "unlocked", adminId);
  const save = await putContent(api, a, page.id, { content: "Mine\n", base_revision: 1, edit_session_id: session.id });
  expect([save.response.status, save.error?.code, save.error?.ended_by]).toEqual([
    409,
    "page.edit_session_unlocked",
    { user_id: adminId, display_name: displayNameOf(adminEmail) },
  ]);
  expect(await readLock(api, a, page.id)).toEqual({ holder: null, expires_in: null });
  expect((await releaseLock(api, admin, page.id)).response.status).toBe(204);
});
