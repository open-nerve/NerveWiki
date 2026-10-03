import { expectAliveSessions } from "../../fixtures/assert/collab";
import { expectSessionGone, sessionsOf } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
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
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

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
