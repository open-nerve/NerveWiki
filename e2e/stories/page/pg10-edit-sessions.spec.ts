import { expectSessionGone } from "../../fixtures/assert/page";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  endSession,
  heartbeat,
  openSession,
  putContent,
  readContent,
  writeContent,
} from "../../fixtures/pages";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { expect, test } from "../../fixtures/test";
import { editStatus, saveEdit, startEditing, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG10, an edit session's lease (M4 design 4; M4/P4 design 3.5): a
// heartbeat moves it on; an ended session is not found; one expired,
// through the database rather than the wall clock (v0.1 design 13.4, item
// 3), refuses a save, and a new one saves. In the browser the editor
// beats on the page's clock, and opens a new session itself (M4/P6
// design 3.6).

test("PG10 (API): a heartbeat moves the lease on; after the end a heartbeat is 404; a session expired refuses a save with 409 page.edit_session_ended, and a new session's save keeps the content whole", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const page = await createPage(api, pat, notebook.id, "Notes");

  const first = await openSession(api, pat, page.id);
  const beat = await heartbeat(api, pat, first.id);
  expect(beat.response.status).toBe(200);
  const [moved] = await db.query<{ later: boolean }>("SELECT $1::timestamptz > $2::timestamptz AS later", [
    beat.data?.expires_at,
    first.expires_at,
  ]);
  expect(moved?.later).toBe(true);
  expect(
    (await writeContent(api, pat, page.id, { content: "One\n", base_revision: 1, edit_session_id: first.id })).revision
  ).toBe(2);

  await db.query(
    "UPDATE edit_sessions SET created_at = now() - interval '2 minutes', expires_at = now() - interval '1 minute' WHERE id = $1",
    [first.id]
  );
  const late = await putContent(api, pat, page.id, { content: "Two\n", base_revision: 2, edit_session_id: first.id });
  expect([late.response.status, late.error?.code]).toEqual([409, "page.edit_session_ended"]);
  expect(await readContent(api, pat, page.id)).toMatchObject({ content: "One\n", revision: 2 });

  const second = await openSession(api, pat, page.id);
  const saved = await writeContent(api, pat, page.id, {
    content: "Two\n",
    base_revision: 2,
    edit_session_id: second.id,
  });
  expect(saved.revision).toBe(3);
  expect(await readContent(api, pat, page.id)).toMatchObject({ content: "Two\n", revision: 3 });

  expect((await endSession(api, pat, second.id)).response.status).toBe(204);
  await expectSessionGone(db, second.id);
  const after = [await heartbeat(api, pat, second.id), await endSession(api, pat, second.id)];
  expect(after.map((a) => [a.response.status, a.error?.code])).toEqual([
    [404, "page.edit_session_not_found"],
    [404, "page.edit_session_not_found"],
  ]);
});

test("PG10 (page): the editor's heartbeat moves the lease on; a session expired meanwhile is opened anew by the save, nothing lost; Done ends the session", async ({
  api,
  db,
  pageWatch,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const notes = await createPage(api, pat, notebook.id, "Notes");
  const page = await signedInPage(tokens);
  await page.clock.install();
  const live = () =>
    db.query<{ id: string; expires_at: string }>(
      "SELECT id, expires_at::text FROM edit_sessions WHERE node_id = $1 AND expires_at > now()",
      [notes.id]
    );

  await page.goto(wikiPagePath(workspace.slug, notebook.id, notes.id));
  await startEditing(page);
  await expect.poll(async () => (await live()).length).toBe(1);
  const [first] = await live();
  const beat = answerTo(page, "POST", `/api/v0/edit-sessions/${first?.id}/heartbeat`);
  await page.clock.fastForward(20_000);
  expect((await beat).status()).toBe(200);
  const [moved] = await db.query<{ later: boolean }>(
    "SELECT expires_at > $2::timestamptz AS later FROM edit_sessions WHERE id = $1",
    [first?.id, first?.expires_at]
  );
  expect(moved?.later).toBe(true);

  await page.keyboard.type("One");
  await saveEdit(page);
  await db.query(
    "UPDATE edit_sessions SET created_at = now() - interval '2 minutes', expires_at = now() - interval '1 minute' WHERE id = $1",
    [first?.id]
  );
  await page.keyboard.type(" two");
  await page.keyboard.press("ControlOrMeta+s");
  await expect(editStatus(page)).toHaveText("Saved.");
  // The save in the expired session is refused, then sent again in a new one.
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  expect(await readContent(api, pat, notes.id)).toMatchObject({ content: "One two", revision: 3 });
  const [second] = await live();
  expect(second?.id).not.toBe(first?.id);

  await page.keyboard.press("ControlOrMeta+e");
  await expect(page.getByRole("article")).toContainText("One two");
  await expect.poll(async () => (await live()).length).toBe(0);
});
