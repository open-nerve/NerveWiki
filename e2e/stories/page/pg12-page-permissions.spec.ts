import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { addNotebookMember } from "../../fixtures/notebook-members";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  endSession,
  getContent,
  getPage,
  getTree,
  heartbeat,
  moveNode,
  openSession,
  postMove,
  postPage,
  postSession,
  putContent,
  renameNode,
  writeContent,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { editorContent, pageHeading, pageTree, quickSwitchFor, wikiPagePath } from "../../fixtures/wiki-pages";
import { newTeam } from "../../fixtures/workspaces";

// PG12, the pages' permissions (M4 design 5): creating, renaming, moving
// and deleting; the content and the edit sessions (M4/P4); in the browser,
// a reader's tree and home without the writes (M4/P5 design 3.7).

test("PG12 (API): a member creates, renames, moves, deletes, writes the content and opens a session by the workspace access editor; a reader reads and cannot write; one who does not see the notebook gets 404; someone else's session is not found", async ({
  api,
  db,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const memberPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "member"), "member");
  const readerEmail = emailFor(testInfo, "reader");
  const readerPat = await joinAs(api, adminPat, workspace.slug, readerEmail, "guest");
  const strangerPat = await joinAs(api, adminPat, workspace.slug, emailFor(testInfo, "stranger"), "guest");
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Team", "editor");
  expect(
    (await addNotebookMember(api, adminPat, notebook.id, await accountIdOf(db, readerEmail), "reader")).response.status
  ).toBe(201);
  const page = await createPage(api, adminPat, notebook.id, "Notes");

  // The member edits by the workspace access alone.
  const created = await postPage(api, memberPat, notebook.id, { parent_id: page.id, title: "Mine" });
  expect(created.response.status).toBe(201);
  expect((await renameNode(api, memberPat, page.id, "Shared")).response.status).toBe(200);
  expect((await writeContent(api, memberPat, page.id, { content: "# Shared\n", base_revision: 1 })).revision).toBe(2);
  const memberSession = await openSession(api, memberPat, page.id);

  // The guest reader reads, and cannot write.
  expect((await getPage(api, readerPat, page.id)).response.status).toBe(200);
  expect((await getTree(api, readerPat, notebook.id)).response.status).toBe(200);
  expect((await getContent(api, readerPat, page.id)).data).toMatchObject({ content: "# Shared\n", revision: 2 });
  const readerCreate = await postPage(api, readerPat, notebook.id, { parent_id: null, title: "Reader's" });
  const readerWrites = [
    readerCreate,
    await renameNode(api, readerPat, page.id, "Reader's"),
    await postMove(api, readerPat, page.id, { parent_id: null, after_id: null }),
    await deleteNode(api, readerPat, page.id),
    await putContent(api, readerPat, page.id, { content: "Reader's", base_revision: 2 }),
    await postSession(api, readerPat, page.id),
  ];
  expect(readerWrites.map((a) => [a.response.status, a.error?.code])).toEqual(
    readerWrites.map(() => [403, "forbidden"])
  );

  // A guest with no role in it sees no notebook: the codes of what each names.
  const strangerCreate = await postPage(api, strangerPat, notebook.id, { parent_id: null, title: "Stranger's" });
  const strangerTree = await getTree(api, strangerPat, notebook.id);
  const strangerRead = await getPage(api, strangerPat, page.id);
  const strangerRename = await renameNode(api, strangerPat, page.id, "Stranger's");
  const strangerMove = await postMove(api, strangerPat, page.id, { parent_id: null });
  const strangerDelete = await deleteNode(api, strangerPat, page.id);
  const strangerContent = [
    await getContent(api, strangerPat, page.id),
    await putContent(api, strangerPat, page.id, { content: "Stranger's", base_revision: 2 }),
    await postSession(api, strangerPat, page.id),
  ];
  expect(
    [strangerCreate, strangerTree, strangerRead, strangerRename, strangerMove, strangerDelete, ...strangerContent].map(
      (a) => [a.response.status, a.error?.code]
    )
  ).toEqual([
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
    ...Array.from({ length: 7 }, () => [404, "page.not_found"]),
  ]);

  // Someone else's session is not found, for its notebook's admin too.
  const othersSession = [
    await heartbeat(api, adminPat, memberSession.id),
    await endSession(api, adminPat, memberSession.id),
    await heartbeat(api, readerPat, memberSession.id),
    await endSession(api, strangerPat, memberSession.id),
  ];
  expect(othersSession.map((a) => [a.response.status, a.error?.code])).toEqual(
    othersSession.map(() => [404, "page.edit_session_not_found"])
  );
  expect((await heartbeat(api, memberPat, memberSession.id)).response.status).toBe(200);

  // The member moves and deletes by the workspace access alone.
  const mine = created.data?.id ?? "";
  expect(await moveNode(api, memberPat, mine, { parent_id: null })).toMatchObject({ name: "Mine", parent_id: null });
  expect((await deleteNode(api, memberPat, mine)).response.status).toBe(204);
  const names = await db.query<{ name: string; deleted: boolean }>(
    "SELECT name, deleted_at IS NOT NULL AS deleted FROM nodes WHERE notebook_id = $1 ORDER BY name",
    [notebook.id]
  );
  expect(names).toEqual([
    { name: "Mine", deleted: true },
    { name: "Shared", deleted: false },
  ]);
});

test("PG12 (page): a reader goes through the notebook's pages without New page, the pages' menus, dragging or Edit; Ctrl+E opens no editor", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat: adminPat, workspace } = await newTeam(api, testInfo);
  const page = await signedInPage(
    await joinOnboarded(api, adminPat, workspace.slug, emailFor(testInfo, "reader"), "member")
  );
  const notebook = await createNotebook(api, adminPat, workspace.slug, "Plans", "viewer");
  const roadmap = await createPage(api, adminPat, notebook.id, "Roadmap");
  const q4 = await createPage(api, adminPat, notebook.id, "Q4", roadmap.id);

  await page.goto(wikiPagePath(workspace.slug, notebook.id, q4.id));
  await expect(pageHeading(page, "Q4")).toBeVisible();
  const tree = pageTree(page, "Plans");
  await expect(tree.getByRole("link")).toHaveText(["Roadmap", "Q4"]);
  await expect(tree.getByRole("button", { name: "New page" })).toHaveCount(0);
  await expect(tree.getByRole("button", { name: /^Actions for / })).toHaveCount(0);
  await expect(tree.locator('[draggable="true"]')).toHaveCount(0);
  await expect(page.getByRole("main").getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
  await page.keyboard.press("ControlOrMeta+e");
  await expect(page.getByRole("article")).toBeAttached();
  await expect(editorContent(page)).toHaveCount(0);

  await page.goto(notebookPath(workspace.slug, notebook.id));
  await expect(notebookHeading(page, "Plans")).toBeVisible();
  await expect(page.getByRole("main").getByRole("button", { name: "New page" })).toHaveCount(0);
  expect(await quickSwitchFor(page, "q4")).toEqual([["Q4", "Roadmap"]]);
});
