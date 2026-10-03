import { accountIdOf } from "../../fixtures/assert/identity";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { countAnswers } from "../../fixtures/browser";
import { holdStream, settleEvents } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
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
import { pageHeading, treeTitles, wikiPagePath } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// C7, the writes pushed (M5 design 4.10): each write of a notebook's pages
// reaches the streams of those who see the notebook as one pages frame, and
// each opening and end of an edit session as a lock frame; the others'
// streams get nothing of it. In the browser (M5/P3 design 3.8, 3.10), what
// a page shows follows them: the reading view, the tree, who is editing.

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

test("C7 (page): B, reading a page of Eng, sees A's writes as A makes them: the content, the tree, and A editing", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat: a, adminEmail, workspace } = await newTeam(api, testInfo);
  const bEmail = emailFor(testInfo, "b");
  const page = await signedInPage(await joinOnboarded(api, a, workspace.slug, bEmail, "member"));
  const eng = await createNotebook(api, a, workspace.slug, "Eng");
  await addedNotebookMember(api, a, eng.id, await accountIdOf(db, bEmail), "reader");
  const notes = await createPage(api, a, eng.id, "Notes", null, "Drafted.\n");
  const other = await createPage(api, a, eng.id, "Other");
  const letStreamIn = await holdStream(page);
  const viewReads = countAnswers(page, "GET", `/api/v0/pages/${notes.id}/view`);
  await page.goto(wikiPagePath(workspace.slug, eng.id, notes.id));
  await expect(pageHeading(page, "Notes")).toBeVisible();
  const content = page.getByRole("article", { name: "Notes" });
  await expect(content).toHaveText("Drafted.");
  await expect.poll(() => treeTitles(page, "Eng")).toEqual(["Notes", "Other"]);
  // B's stream connects, and its refresh reads the page again: from then on, A's writes come as events.
  letStreamIn();
  await expect.poll(viewReads).toBeGreaterThanOrEqual(2);

  await writeContent(api, a, notes.id, { content: "Drafted.\n\nPushed.\n", base_revision: 1 });
  await expect(content).toContainText("Pushed.");

  const fresh = await createPage(api, a, eng.id, "Fresh");
  await expect.poll(() => treeTitles(page, "Eng")).toEqual(["Notes", "Other", "Fresh"]);
  expect((await renameNode(api, a, other.id, "Renamed")).response.status).toBe(200);
  await expect.poll(() => treeTitles(page, "Eng")).toEqual(["Notes", "Renamed", "Fresh"]);
  await moveNode(api, a, fresh.id, { parent_id: null, after_id: null });
  await expect.poll(() => treeTitles(page, "Eng")).toEqual(["Fresh", "Notes", "Renamed"]);
  expect((await deleteNode(api, a, other.id)).response.status).toBe(204);
  await expect.poll(() => treeTitles(page, "Eng")).toEqual(["Fresh", "Notes"]);

  const editing = page.getByText(`${displayNameOf(adminEmail)} is editing this page.`, { exact: true });
  const session = await openSession(api, a, notes.id);
  await expect(editing).toBeVisible();
  expect((await endSession(api, a, session.id)).response.status).toBe(204);
  await expect(editing).toBeHidden();
});
