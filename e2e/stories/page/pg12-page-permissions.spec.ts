import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor } from "../../fixtures/auth";
import { joinAs } from "../../fixtures/invitations";
import { addNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  getPage,
  getTree,
  moveNode,
  postMove,
  postPage,
  renameNode,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG12, the pages' permissions (M4 design 5): creating, renaming, moving
// and deleting; the content comes with M4/P4.

test("PG12 (API): a member creates, renames, moves and deletes by the workspace access editor; a reader reads and cannot write; one who does not see the notebook gets 404", async ({
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

  // The guest reader reads, and cannot write.
  expect((await getPage(api, readerPat, page.id)).response.status).toBe(200);
  expect((await getTree(api, readerPat, notebook.id)).response.status).toBe(200);
  const readerCreate = await postPage(api, readerPat, notebook.id, { parent_id: null, title: "Reader's" });
  const readerWrites = [
    readerCreate,
    await renameNode(api, readerPat, page.id, "Reader's"),
    await postMove(api, readerPat, page.id, { parent_id: null, after_id: null }),
    await deleteNode(api, readerPat, page.id),
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
  expect(
    [strangerCreate, strangerTree, strangerRead, strangerRename, strangerMove, strangerDelete].map((a) => [
      a.response.status,
      a.error?.code,
    ])
  ).toEqual([
    [404, "notebook.not_found"],
    [404, "notebook.not_found"],
    [404, "page.not_found"],
    [404, "page.not_found"],
    [404, "page.not_found"],
    [404, "page.not_found"],
  ]);

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
