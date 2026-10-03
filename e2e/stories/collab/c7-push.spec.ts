import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor } from "../../fixtures/auth";
import { settleEvents } from "../../fixtures/events";
import { joinAs } from "../../fixtures/invitations";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  endSession,
  moveNode,
  openSession,
  renameNode,
  writeContent,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// C7, the writes pushed (M5 design 4.10): each write of a notebook's pages
// reaches the streams of those who see the notebook as one pages frame, and
// each opening and end of an edit session as a lock frame; the others'
// streams get nothing of it.

test("C7 (API): B, who sees Eng, receives a pages frame of each of A's writes and a lock frame of each session change; C, who does not, receives nothing of them", async ({
  api,
  db,
  nervewiki,
  openEvents,
}, testInfo) => {
  const { pat: a, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const b = await joinAs(api, a, workspace.slug, bEmail, "member");
  const c = await joinAs(api, a, workspace.slug, emailFor(testInfo, "c"), "member");
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
  const mine = await createNotebook(api, c, workspace.slug, "Mine");
  const control = await createPage(api, c, mine.id, "Control");
  await settleEvents(api, nervewiki.baseURL, a, (await createPage(api, a, eng.id, "Marker")).id);
  const bStream = await openEvents(b);
  const cStream = await openEvents(c);
  const ofEng = { workspace_id: workspace.id, notebook_id: eng.id };

  const page = await createPage(api, a, eng.id, "Notes");
  expect(await bStream.expectNext("pages")).toEqual({ ...ofEng, tree: true, pages: [{ id: page.id, revision: 1 }] });
  const parent = await createPage(api, a, eng.id, "Parent", null, "# Parent\n");
  expect(await bStream.expectNext("pages")).toEqual({ ...ofEng, tree: true, pages: [{ id: parent.id, revision: 1 }] });
  expect((await renameNode(api, a, page.id, "Minutes")).response.status).toBe(200);
  expect(await bStream.expectNext("pages")).toEqual({ ...ofEng, tree: true, pages: [] });
  await moveNode(api, a, page.id, { parent_id: parent.id });
  expect(await bStream.expectNext("pages")).toEqual({ ...ofEng, tree: true, pages: [] });
  await writeContent(api, a, page.id, { content: "# Minutes\n", base_revision: 1 });
  expect(await bStream.expectNext("pages")).toEqual({ ...ofEng, tree: false, pages: [{ id: page.id, revision: 2 }] });

  const session = await openSession(api, a, page.id);
  expect(await bStream.expectNext("lock")).toEqual({ ...ofEng, page_id: page.id, session_id: session.id });
  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  expect(await bStream.expectNext("lock")).toEqual({ ...ofEng, page_id: page.id, session_id: session.id });

  expect((await deleteNode(api, a, parent.id)).response.status).toBe(204);
  expect(await bStream.expectNext("pages")).toEqual({ ...ofEng, tree: true, pages: [] });

  // C's first frame since it opened is that of its own write, after all of A's.
  await writeContent(api, c, control.id, { content: "# Control\n", base_revision: 1 });
  expect(await cStream.expectNext("pages")).toEqual({
    workspace_id: workspace.id,
    notebook_id: mine.id,
    tree: false,
    pages: [{ id: control.id, revision: 2 }],
  });
});
