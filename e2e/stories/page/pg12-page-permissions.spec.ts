import { accountIdOf } from "../../fixtures/assert/identity";
import { emailFor } from "../../fixtures/auth";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, postMcp, readPage, resultOf } from "../../fixtures/mcp";
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
// a reader's tree and home without the writes (M4/P5 design 3.7). Through
// MCP, a notebook not seen is the notebook entry's 404 and every path's
// notebook.not_found at the account entry (M9/P5 design 3.2).

/** A write's answer, as much as PG12 reads of it. */
interface Written {
  node_id: string;
  version: string;
  created: { path: string; node_id: string }[];
}

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

test("PG12 (MCP): a member's agent writes, renames, moves and deletes by the workspace access editor; a guest reader's reads and is refused each write; one who does not see the notebook is the notebook entry's 404 and the account entry's notebook.not_found", async ({
  api,
  db,
  nervewiki,
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
  const member = await mcpClient(nervewiki.baseURL, memberPat, { notebook: notebook.id });
  const reader = await mcpClient(nervewiki.baseURL, readerPat, { notebook: notebook.id });
  const stranger = await mcpClient(nervewiki.baseURL, strangerPat);

  // The member's agent edits by the workspace access alone: Mine goes with Notes, renamed Shared.
  const created = resultOf<Written>(await member.call("write", { path: "Notes/Mine.md", content: "" }), "write Mine");
  expect(created.created.map((c) => c.path)).toEqual(["Notes/Mine.md"]);
  const renamed = resultOf(await member.call("move", { from: "Notes.md", to: "Shared.md" }), "rename Notes");
  expect(renamed).toMatchObject({ path: "Shared.md", node_id: page.id, changed: true });
  const { version } = (await readPage(member, { path: "Shared.md" })).meta;
  expect(version).toBe(`${page.id}:1`);
  const shared = resultOf<Written>(
    await member.call("write", { path: "Shared.md", content: "# Shared\n", base_version: version }),
    "write Shared"
  );
  expect(shared.version).toBe(`${page.id}:2`);

  // The guest reader's agent reads, and is refused each write the member's would make, on the version it read.
  const read = await readPage(reader, { path: "Shared.md" });
  expect([read.text, read.meta.version]).toEqual(["# Shared\n", `${page.id}:2`]);
  const listed = resultOf<{ items: { path: string }[] }>(await reader.call("list"), "the reader's list");
  expect(listed.items.map((item) => item.path)).toEqual(["Shared.md"]);
  const base = read.meta.version;
  const readerWrites = await Promise.all([
    reader.call("write", { path: "Reader's.md", content: "" }),
    reader.call("move", { from: "Shared.md", to: "Reader's rename.md" }),
    reader.call("delete", { path: "Shared/Mine.md" }),
    reader.call("write", { path: "Shared.md", content: "# Reader\n", base_version: base }),
    reader.call("edit", { path: "Shared.md", old_string: "# Shared", new_string: "# Reader", base_version: base }),
    reader.call("batch", { message: "reader", ops: [{ op: "write", path: "Batch.md", content: "" }] }),
  ]);
  expect(readerWrites.map((answer, i) => errorOf(answer, `the reader's write ${i}`).code)).toEqual([
    "forbidden",
    "forbidden",
    "forbidden",
    "forbidden",
    "forbidden",
    "forbidden",
  ]);

  // A guest with no role in it sees no notebook: the notebook entry is not found, before the SDK; at the account
  // entry no notebook is listed, and each tool there is notebook.not_found, the notebook decided before the path.
  const initialize = {
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: { protocolVersion: "2025-11-25", capabilities: {}, clientInfo: { name: "e2e-agent", version: "1" } },
  };
  const entry = await postMcp(nervewiki.baseURL, strangerPat, initialize, notebook.id);
  expect([entry.status, entry.headers.get("content-type"), entry.headers.get("cache-control")]).toEqual([
    404,
    "application/problem+json",
    "no-store",
  ]);
  expect(((await entry.json()) as { code: string }).code).toBe("notebook.not_found");
  expect(resultOf(await stranger.call("notebooks"), "the stranger's notebooks")).toEqual({
    notebooks: [],
    truncated: false,
  });
  const strangerCalls = [
    ["list", {}],
    ["read", { path: "Shared.md" }],
    ["write", { path: "Stranger's.md", content: "" }],
    ["move", { from: "Shared.md", to: "Stranger's.md" }],
    ["delete", { path: "Shared.md" }],
    ["edit", { path: "Shared.md", old_string: "# Shared", new_string: "# Stranger" }],
    ["batch", { message: "stranger", ops: [{ op: "write", path: "Batch.md", content: "" }] }],
  ] as const;
  const strangers = await Promise.all(
    strangerCalls.map(([tool, args]) => stranger.call(tool, { notebook: notebook.id, ...args }))
  );
  expect(
    strangers.map((answer, i) => [strangerCalls[i]?.[0], errorOf(answer, `the stranger's call ${i}`).code])
  ).toEqual(strangerCalls.map(([tool]) => [tool, "notebook.not_found"]));

  // The member's agent moves and deletes by the workspace access alone; no write of the others left a node.
  const moved = resultOf(await member.call("move", { from: "Shared/Mine.md", to: "Mine.md" }), "move Mine");
  expect(moved).toMatchObject({ path: "Mine.md", node_id: created.node_id, changed: true });
  const deleted = resultOf<{ node_id: string; pages: number; attachments: number }>(
    await member.call("delete", { path: "Mine.md" }),
    "delete Mine"
  );
  expect([deleted.node_id, deleted.pages, deleted.attachments]).toEqual([created.node_id, 1, 0]);
  const names = await db.query<{ name: string; deleted: boolean }>(
    "SELECT name, deleted_at IS NOT NULL AS deleted FROM nodes WHERE notebook_id = $1 ORDER BY name",
    [notebook.id]
  );
  expect(names).toEqual([
    { name: "Mine", deleted: true },
    { name: "Shared", deleted: false },
  ]);
});

test("PG12 (page): a reader goes through the notebook's pages without New page, the pages' menus, dragging or Edit; Ctrl+E opens no editor; the address of a page in a notebook it does not see is no page", async ({
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

  const secret = await createNotebook(api, adminPat, workspace.slug, "Secret", "none");
  const hidden = await createPage(api, adminPat, secret.id, "Hidden");
  await page.goto(wikiPagePath(workspace.slug, secret.id, hidden.id));
  await expect(page.getByRole("heading", { level: 1, name: "Page not found", exact: true })).toBeVisible();
  await expect(pageTree(page, "Secret")).toHaveCount(0);
  await expect(pageHeading(page, "Hidden")).toHaveCount(0);
});
