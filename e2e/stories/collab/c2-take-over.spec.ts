import { expectAliveSessions, expectTombstone } from "../../fixtures/assert/collab";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { postTakeOver, takeOver } from "../../fixtures/collab";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, heartbeat, openSession, postSession, putContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// C2, taking one's own edit over (M5 design 4.2, 4.3): the same account's
// second opening is refused, naming itself; one that takes over ends the
// first session, which then says why. Someone else's lock a take-over
// does not pass.

test("C2 (API): A's second opening is 409 page.locked naming A, and B's take-over too; A's with take_over opens, and the first session's heartbeat and save are 409 page.edit_session_taken_over; its end is 204", async ({
  api,
  db,
}, testInfo) => {
  const { adminEmail, adminId, pat: a, workspace } = await newTeam(api, testInfo);
  const b = await joinAs(api, a, workspace.slug, emailFor(testInfo, "b"), "member");
  const notebook = await createNotebook(api, a, workspace.slug, "Plans", "editor");
  const page = await createPage(api, a, notebook.id, "Notes");

  const first = await openSession(api, a, page.id);
  const lock = { page_id: page.id, user_id: adminId, display_name: displayNameOf(adminEmail) };
  const refused = [await postSession(api, a, page.id), await postTakeOver(api, b, page.id)];
  expect(refused.map((r) => [r.response.status, r.error?.code, r.error?.lock])).toEqual([
    [409, "page.locked", lock],
    [409, "page.locked", lock],
  ]);
  await expectAliveSessions(db, page.id, [first.id]);

  const second = await takeOver(api, a, page.id);
  await expectTombstone(db, first.id, "taken_over", adminId);
  await expectAliveSessions(db, page.id, [second.id]);
  const beat = await heartbeat(api, a, first.id);
  const save = await putContent(api, a, page.id, { content: "Mine\n", base_revision: 1, edit_session_id: first.id });
  expect([beat, save].map((r) => [r.response.status, r.error?.code])).toEqual([
    [409, "page.edit_session_taken_over"],
    [409, "page.edit_session_taken_over"],
  ]);
  expect((await endSession(api, a, first.id)).response.status).toBe(204);
  await expectAliveSessions(db, page.id, [second.id]);
});
