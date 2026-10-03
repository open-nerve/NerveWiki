import { aliveSessionsOf, expectAliveSessions, expectTombstone } from "../../fixtures/assert/collab";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { postTakeOver, takeOver } from "../../fixtures/collab";
import { joinAs } from "../../fixtures/invitations";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  endSession,
  heartbeat,
  openSession,
  postSession,
  putContent,
  readContent,
} from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import {
  editorContent,
  editRefused,
  lostBanner,
  saveEdit,
  startEditing,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

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

test("C2 (page): A edits Notes in one tab; in a second, Edit says A edits it elsewhere, and Edit here takes it over: the first tab hears it as an event, read-only, saying why, its saved text kept", async ({
  anotherTab,
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { adminId, pat: a, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, a, workspace.slug, "Plans");
  const notes = await createPage(api, a, notebook.id, "Notes", null, "Drafted.\n");
  const path = wikiPagePath(workspace.slug, notebook.id, notes.id);
  const first = await signedInPage(tokens);
  await first.clock.install();

  await first.goto(path);
  await startEditing(first);
  await first.keyboard.press("ControlOrMeta+End");
  await first.keyboard.type("One");
  await saveEdit(first);
  const [held] = await aliveSessionsOf(db, notes.id);
  // The first tab beats now: its next beat is 20 seconds off, so that what it hears sooner comes as an event.
  const beat = answerTo(first, "POST", `/api/v0/edit-sessions/${held}/heartbeat`);
  await first.clock.fastForward(20_000);
  expect((await beat).status()).toBe(200);

  const second = await anotherTab(first);
  await second.goto(path);
  await editRefused(second, "You are editing this page elsewhere.");
  watchOf(second).expectConsole({ errors: [failedToLoad(409)] });
  await second.getByRole("main").getByRole("button", { name: "Edit here", exact: true }).click();
  await expect(editorContent(second)).toBeFocused();

  await expect(lostBanner(first)).toContainText("You went on editing this page elsewhere: this editor saves no more.");
  await expect(lostBanner(first)).toBeFocused();
  await expect(editorContent(first)).toHaveAttribute("contenteditable", "false");
  await expect(editorContent(first)).toContainText("One");
  // The first session's beat on the event: page.edit_session_taken_over.
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  await expectTombstone(db, held ?? "", "taken_over", adminId);
  const alive = await aliveSessionsOf(db, notes.id);
  expect(alive).toHaveLength(1);
  expect(alive).not.toEqual([held]);
  expect((await readContent(api, a, notes.id)).content).toBe("Drafted.\nOne");
});
