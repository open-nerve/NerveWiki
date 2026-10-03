import { expectAliveSessions } from "../../fixtures/assert/collab";
import { accountIdOf } from "../../fixtures/assert/identity";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { readLock } from "../../fixtures/collab";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, openSession, postSession } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// C1, one editor at a time (M5 design 4.1): a page's alive edit session is
// its lock; another opening is refused, naming its holder, until it ends.

test("C1 (API): while A edits a page, B's opening is 409 page.locked naming A and the page; once A ends, B opens; the lock's read names A, then B", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const bId = await accountIdOf(db, bEmail);
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const page = await createPage(api, a, notebook.id, "Notes");

  const held = await openSession(api, a, page.id);
  const refused = await postSession(api, b, page.id);
  expect([refused.response.status, refused.error?.code, refused.error?.lock]).toEqual([
    409,
    "page.locked",
    { page_id: page.id, user_id: adminId, display_name: displayNameOf(adminEmail) },
  ]);
  const byA = await readLock(api, b, page.id);
  expect(byA.holder).toEqual({ user_id: adminId, display_name: displayNameOf(adminEmail) });
  expect(byA.expires_in).toBeGreaterThan(110);
  expect(byA.expires_in).toBeLessThanOrEqual(120);
  await expectAliveSessions(db, page.id, [held.id]);

  expect((await endSession(api, a, held.id)).response.status).toBe(204);
  expect(await readLock(api, b, page.id)).toEqual({ holder: null, expires_in: null });
  const opened = await openSession(api, b, page.id);
  expect((await readLock(api, a, page.id)).holder).toEqual({ user_id: bId, display_name: displayNameOf(bEmail) });
  await expectAliveSessions(db, page.id, [opened.id]);
});
