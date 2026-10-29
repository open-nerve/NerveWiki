import type { ApiClient, NodeMove } from "@nervewiki/api-client";
import type { Page as BrowserPage } from "@playwright/test";

import { expectAssetsDeletedWithNodes, expectUploaded, storedBlobs } from "../../fixtures/assert/asset";
import { accountIdOf } from "../../fixtures/assert/identity";
import {
  expectChangesetOf,
  expectContentWritten,
  expectDeletionRevisions,
  expectNewPage,
  expectRelocated,
  expectRenamed,
  expectSubtreeDeleted,
  expectSourceHashes,
  pageStateOf,
  placeOf,
} from "../../fixtures/assert/page";
import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { createToken, emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, mcpClient, readPage, resultOf, writtenOf, type McpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  deleteNode,
  listNodes,
  moveNode,
  postMove,
  postSession,
  putContent,
  renameNode,
  writeContent,
} from "../../fixtures/pages";
import { expect, test, watchOf } from "../../fixtures/test";
import { choosePageAction, dragPage, pageHeading, pageTree, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// LW5, source management (M10/P2 design 5): writers add sources, while
// admins rename, move, reorder and delete existing sources. Identity is
// the configured source root, even after its rename; a nested AGENTS is
// a source, not the protected root instruction page. MCP has no reorder
// parameter or tool: real ordering is covered by the API cases and Go's
// source permission matrix, not by a path-preserving MCP no-op.

interface Moved {
  path: string;
  node_id: string;
  changed: boolean;
  changeset_id: string | null;
  at: string | null;
}

interface Deleted {
  path: string;
  node_id: string;
  pages: number;
  attachments: number;
  changeset_id: string;
}

interface Written {
  node_id: string;
  version: string;
  changed: boolean;
  changeset_id: string | null;
  at: string | null;
}

/** The source root is found by its configured id, not by a descendant's name. */
async function sourceWiki(api: ApiClient, pat: string, slug: string) {
  const notebook = await createNotebook(api, pat, slug, "Sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const raw = (await listNodes(api, pat, notebook.id)).find((node) => node.id === notebook.source_root_id);
  if (raw?.name !== "raw") throw new Error("the template did not return its configured source root");
  return { notebook, raw };
}

/** Every refusal is a separate request bracketed by the persisted notebook snapshot. */
async function refusedAPI(
  db: Database,
  notebookId: string,
  call: () => Promise<{ response: Response; error?: { code?: string } }>,
  code = "page.source_admin_only"
) {
  const before = await pageStateOf(db, notebookId);
  const answer = await call();
  expect([answer.response.status, answer.error?.code]).toEqual([code === "page.source_admin_only" ? 403 : 409, code]);
  expect(await pageStateOf(db, notebookId)).toEqual(before);
}

/** Content bytes and versions, including each attachment's blob, through management and soft deletion. */
async function bodiesOf(db: Database, notebookId: string) {
  const rows = await db.query(
    `SELECT n.id, n.kind, p.content, p.revision, p.byte_size AS page_bytes,
            encode(p.content_hash, 'hex') AS page_hash, b.id AS blob_id, b.byte_size AS asset_bytes,
            encode(b.sha256, 'hex') AS asset_hash, b.mime, b.width, b.height
       FROM nodes n LEFT JOIN page_contents p ON p.node_id = n.id LEFT JOIN asset_blobs b ON b.node_id = n.id
      WHERE n.notebook_id = $1 ORDER BY n.id, b.id LIMIT 65`,
    [notebookId]
  );
  expect(rows.length, "management assertions require a small notebook").toBeLessThanOrEqual(64);
  return rows;
}

/** The last actual management changeset of this node, not an unrelated notebook-wide latest entry. */
async function latestManagement(db: Database, id: string): Promise<string> {
  const [row] = await db.query<{ id: string }>(
    `SELECT s.id FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id
      WHERE i.node_id = $1 ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
    [id]
  );
  if (!row) throw new Error(`no management changeset of ${id}`);
  return row.id;
}

for (const kind of ["page", "asset"] as const) {
  test(`LW5 (API): only an admin renames, reorders, moves within and out of the renamed source root, and deletes an existing source ${kind}`, async ({
    api,
    db,
    nervewiki,
  }, testInfo) => {
    const { adminId, pat, workspace } = await newTeam(api, testInfo);
    const writerEmail = emailFor(testInfo, "writer");
    const writerPat = await joinAs(api, pat, workspace.slug, writerEmail, "member");
    const writerId = await accountIdOf(db, writerEmail);
    const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
    const rootBodies = await bodiesOf(db, notebook.id);
    expect((await renameNode(api, pat, raw.id, "Evidence")).response.status).toBe(200);
    await expectRenamed(db, raw.id, "raw", "Evidence", adminId);
    expect(await bodiesOf(db, notebook.id)).toEqual(rootBodies);

    // Both levels are created under the same configured id after its rename.
    const left = await createPage(api, writerPat, notebook.id, "Left", raw.id);
    const right = await createPage(api, writerPat, notebook.id, "Right", raw.id);
    const earlier = await createPage(api, writerPat, notebook.id, "Earlier", left.id);
    const source =
      kind === "page"
        ? await createPage(api, writerPat, notebook.id, "Source", left.id, "Original evidence.\n")
        : await uploadAsset(api, writerPat, notebook.id, { name: "source.png", bytes: pngBytes }, left.id);
    const child =
      kind === "page" ? await createPage(api, writerPat, notebook.id, "Child", source.id, "Child evidence.\n") : null;
    if ("content_url" in source) {
      await expectUploaded(db, nervewiki.storageDir, source, pngBytes, writerId, "api");
    } else {
      await expectNewPage(db, source, writerId, "api", "Original evidence.\n");
    }
    const bodies = await bodiesOf(db, notebook.id);
    const renamed = kind === "page" ? "Renamed source" : "renamed.png";
    const was = { ...(await placeOf(db, source.id)), name: source.name };

    await refusedAPI(db, notebook.id, () => renameNode(api, writerPat, source.id, renamed));
    expect((await renameNode(api, pat, source.id, renamed)).response.status).toBe(200);
    await expectRelocated(db, source.id, was, { parentId: left.id, name: renamed }, adminId);
    await expectChangesetOf(db, await latestManagement(db, source.id), {
      client: "api",
      byId: adminId,
      items: [source.id],
      versions: [],
    });
    expect(await bodiesOf(db, notebook.id)).toEqual(bodies);

    const move = async (body: NodeMove) => {
      const from = { ...(await placeOf(db, source.id)), name: renamed };
      const moved = await moveNode(api, pat, source.id, body);
      expect([moved.id, moved.name, moved.parent_id]).toEqual([source.id, renamed, body.parent_id]);
      await expectRelocated(db, source.id, from, { parentId: body.parent_id, name: renamed }, adminId);
      await expectChangesetOf(db, await latestManagement(db, source.id), {
        client: "api",
        byId: adminId,
        items: [source.id],
        versions: [],
      });
      expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
    };

    const siblings = (await listNodes(api, pat, notebook.id)).filter((node) => node.parent_id === left.id);
    expect(siblings.map((node) => node.id)).toEqual([earlier.id, source.id]);
    const beforeOrder = await placeOf(db, source.id);
    const first = { parent_id: left.id, after_id: null };
    await refusedAPI(db, notebook.id, () => postMove(api, writerPat, source.id, first));
    await move(first);
    expect((await placeOf(db, source.id)).sortOrder).not.toBe(beforeOrder.sortOrder);
    expect(
      (await listNodes(api, pat, notebook.id)).filter((node) => node.parent_id === left.id).map((node) => node.id)
    ).toEqual([source.id, earlier.id]);

    await refusedAPI(db, notebook.id, () => postMove(api, writerPat, source.id, { parent_id: right.id }));
    await move({ parent_id: right.id });
    await refusedAPI(db, notebook.id, () => postMove(api, writerPat, source.id, { parent_id: null }));
    await move({ parent_id: null });
    await move({ parent_id: left.id });
    await refusedAPI(db, notebook.id, () => deleteNode(api, writerPat, source.id));
    expect((await deleteNode(api, pat, source.id)).response.status).toBe(204);
    const deletedIds = child === null ? [source.id] : [source.id, child.id];
    const deletion = await latestManagement(db, source.id);
    await expectChangesetOf(db, deletion, { client: "api", byId: adminId, items: deletedIds, versions: [] });
    await expectDeletionRevisions(db, deletion, deletedIds);
    if (kind === "page") {
      await expectSubtreeDeleted(db, source.id, deletedIds, adminId);
    } else {
      await expectAssetsDeletedWithNodes(db, [source.id]);
      expect(
        await db.query("SELECT updated_by_id FROM nodes WHERE id = $1 AND deleted_at = updated_at", [source.id])
      ).toEqual([{ updated_by_id: adminId }]);
    }
    expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
  });
}

test("LW5 (API): a writer renames and moves an ordinary page into sources, which becomes immutable until an admin moves it out; nested AGENTS is an ordinary source", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const writerPat = await joinAs(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
  const folder = await createPage(api, writerPat, notebook.id, "Folder", raw.id);
  const ordinary = await createPage(api, writerPat, notebook.id, "Ordinary", null, "Ordinary evidence.\n");
  const nested = await createPage(api, writerPat, notebook.id, "AGENTS", folder.id, "Nested evidence.\n");
  await expectNewPage(db, nested, writerId, "api", "Nested evidence.\n");
  const bodies = await bodiesOf(db, notebook.id);

  // REST has separate rename and move endpoints; MCP below proves one combined move+rename.
  expect((await renameNode(api, writerPat, ordinary.id, "Entered")).response.status).toBe(200);
  await expectRenamed(db, ordinary.id, "Ordinary", "Entered", writerId);
  expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
  const outside = { ...(await placeOf(db, ordinary.id)), name: "Entered" };
  await moveNode(api, writerPat, ordinary.id, { parent_id: folder.id });
  await expectRelocated(db, ordinary.id, outside, { parentId: folder.id, name: "Entered" }, writerId);
  expect(await bodiesOf(db, notebook.id)).toEqual(bodies);

  await refusedAPI(
    db,
    notebook.id,
    () => putContent(api, writerPat, ordinary.id, { content: "Changed inside.\n", base_revision: 1 }),
    "page.source_immutable"
  );
  await refusedAPI(db, notebook.id, () => postSession(api, writerPat, ordinary.id), "page.source_immutable");
  await refusedAPI(db, notebook.id, () => postMove(api, writerPat, ordinary.id, { parent_id: null }));
  const inside = { ...(await placeOf(db, ordinary.id)), name: "Entered" };
  await moveNode(api, pat, ordinary.id, { parent_id: null });
  await expectRelocated(db, ordinary.id, inside, { parentId: null, name: "Entered" }, adminId);
  expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
  const content = "Writable outside again.\n";
  const written = await writeContent(api, writerPat, ordinary.id, { content, base_revision: 1 });
  expect(written.revision).toBe(2);
  await expectContentWritten(db, written, content, writerId);

  await refusedAPI(
    db,
    notebook.id,
    () => putContent(api, writerPat, nested.id, { content: "Changed nested rules.\n", base_revision: 1 }),
    "page.source_immutable"
  );
  await refusedAPI(db, notebook.id, () => renameNode(api, writerPat, nested.id, "Archived rules"));
  await refusedAPI(db, notebook.id, () => deleteNode(api, writerPat, nested.id));
  const beforeNested = await bodiesOf(db, notebook.id);
  expect((await renameNode(api, pat, nested.id, "Archived rules")).response.status).toBe(200);
  await expectRenamed(db, nested.id, "AGENTS", "Archived rules", adminId);
  expect(await bodiesOf(db, notebook.id)).toEqual(beforeNested);
  expect((await deleteNode(api, pat, nested.id)).response.status).toBe(204);
  await expectSubtreeDeleted(db, nested.id, [nested.id], adminId);
  expect(await bodiesOf(db, notebook.id)).toEqual(beforeNested);
});

for (const scope of ["notebook", "account"] as const) {
  test(`LW5 (MCP): ${scope} scope requires an admin for source page and attachment management, moving an ordinary page in changes its write boundary, and nested AGENTS has source identity`, async ({
    api,
    db,
    nervewiki,
  }, testInfo) => {
    const { adminId, pat, workspace } = await newTeam(api, testInfo);
    const writerEmail = emailFor(testInfo, "writer");
    const writerPat = await joinAs(api, pat, workspace.slug, writerEmail, "member");
    const writerId = await accountIdOf(db, writerEmail);
    const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
    const left = await createPage(api, writerPat, notebook.id, "Left", raw.id);
    const right = await createPage(api, writerPat, notebook.id, "Right", raw.id);
    const source = await createPage(api, writerPat, notebook.id, "Source", left.id, "Original evidence.\n");
    const child = await createPage(api, writerPat, notebook.id, "Child", source.id, "Child evidence.\n");
    const asset = await uploadAsset(api, writerPat, notebook.id, { name: "source.png", bytes: pngBytes }, left.id);
    await expectUploaded(db, nervewiki.storageDir, asset, pngBytes, writerId, "api");
    const ordinary = await createPage(api, writerPat, notebook.id, "Ordinary", null, "Ordinary evidence.\n");

    // Independent PATs keep each initialized client's recorded identity separate, in both scopes.
    const writerToken = await createToken(api, writerPat, { name: `LW5 ${scope} writer` });
    const adminToken = await createToken(api, pat, { name: `LW5 ${scope} admin` });
    const entry = scope === "notebook" ? { notebook: notebook.id } : {};
    const writerName = `lw5-${scope}-writer`;
    const adminName = `lw5-${scope}-admin`;
    const writer = await mcpClient(nervewiki.baseURL, writerToken.token, { ...entry, client: writerName });
    const admin = await mcpClient(nervewiki.baseURL, adminToken.token, { ...entry, client: adminName });
    const args = (body: Record<string, unknown>) => (scope === "account" ? { ...body, notebook: notebook.id } : body);
    const clientOf = (agent: McpClient) => `mcp:${agent === admin ? adminName : writerName}`;

    const refuse = async (tool: string, body: Record<string, unknown>, code = "page.source_admin_only") => {
      const before = await pageStateOf(db, notebook.id);
      expect(errorOf(await writer.call(tool, args(body)), `${scope}: ${tool} ${JSON.stringify(body)}`).code).toBe(code);
      expect(await pageStateOf(db, notebook.id)).toEqual(before);
    };
    const move = async (
      agent: McpClient,
      byId: string,
      node: { id: string; name: string },
      from: string,
      to: string,
      parentId: string | null,
      name: string
    ) => {
      const before = { ...(await placeOf(db, node.id)), name: node.name };
      const bodies = await bodiesOf(db, notebook.id);
      const moved = resultOf<Moved>(await agent.call("move", args({ from, to })), `move ${from} to ${to}`);
      expect([moved.node_id, moved.path, moved.changed]).toEqual([node.id, to, true]);
      await expectRelocated(db, node.id, before, { parentId, name }, byId, String(moved.at));
      await expectChangesetOf(db, String(moved.changeset_id), {
        client: clientOf(agent),
        byId,
        items: [node.id],
        versions: [],
      });
      expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
      return { id: node.id, name };
    };

    await move(admin, adminId, raw, "raw.md", "Evidence.md", null, "Evidence");

    const manage = async (node: { id: string; name: string }, kind: "page" | "asset") => {
      const suffix = kind === "page" ? ".md" : "";
      const renamed = kind === "page" ? "Renamed" : "renamed.png";
      const returned = kind === "page" ? "Returned" : "returned.png";
      const original = `Evidence/Left/${node.name}${suffix}`;
      const sameParent = `Evidence/Left/${renamed}${suffix}`;
      const otherParent = `Evidence/Right/${renamed}${suffix}`;
      const both = `Evidence/Left/${returned}${suffix}`;
      const outside = `${returned}${suffix}`;

      await refuse("move", { from: original, to: sameParent });
      const renamedNode = await move(admin, adminId, node, original, sameParent, left.id, renamed);
      await refuse("move", { from: sameParent, to: otherParent });
      await move(admin, adminId, renamedNode, sameParent, otherParent, right.id, renamed);
      await refuse("move", { from: otherParent, to: both });
      const returnedNode = await move(admin, adminId, renamedNode, otherParent, both, left.id, returned);
      await refuse("move", { from: both, to: outside });
      await move(admin, adminId, returnedNode, both, outside, null, returned);
      await move(admin, adminId, returnedNode, outside, both, left.id, returned);

      // The page has a real child: recursive reaches the source rule, rather than the parameter refusal.
      await refuse("delete", { path: both, recursive: true });
      const bodies = await bodiesOf(db, notebook.id);
      const deleted = resultOf<Deleted>(
        await admin.call("delete", args({ path: both, recursive: true })),
        `delete source ${kind}`
      );
      expect([deleted.path, deleted.node_id, deleted.pages, deleted.attachments]).toEqual([
        both,
        node.id,
        kind === "page" ? 2 : 0,
        kind === "asset" ? 1 : 0,
      ]);
      const ids = kind === "page" ? [node.id, child.id] : [node.id];
      await expectChangesetOf(db, deleted.changeset_id, {
        client: clientOf(admin),
        byId: adminId,
        items: ids,
        versions: [],
      });
      await expectDeletionRevisions(db, deleted.changeset_id, ids);
      if (kind === "page") await expectSubtreeDeleted(db, node.id, ids, adminId);
      else await expectAssetsDeletedWithNodes(db, ids);
      expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
    };
    await manage(source, "page");
    await manage(asset, "asset");

    // A writer moves and renames an ordinary page into the renamed source root in one real MCP call.
    const entered = await move(
      writer,
      writerId,
      ordinary,
      "Ordinary.md",
      "Evidence/Left/Entered.md",
      left.id,
      "Entered"
    );
    const read = await readPage(writer, args({ path: "Evidence/Left/Entered.md" }));
    expect(read.text).toBe("Ordinary evidence.\n");
    await refuse(
      "write",
      { path: "Evidence/Left/Entered.md", content: "Changed inside.\n", base_version: read.meta.version },
      "page.source_immutable"
    );
    await refuse("move", { from: "Evidence/Left/Entered.md", to: "Released.md" });
    await move(admin, adminId, entered, "Evidence/Left/Entered.md", "Released.md", null, "Released");
    const released = resultOf<Written>(
      await writer.call("edit", args({ path: "Released.md", old_string: "Ordinary", new_string: "Released" })),
      "edit the released page"
    );
    expect(released.changed).toBe(true);
    const releasedPage = writtenOf(released, "edit the released page");
    expect([releasedPage.id, releasedPage.revision]).toEqual([ordinary.id, 2]);
    await expectContentWritten(db, releasedPage, "Released evidence.\n", writerId);
    await expectChangesetOf(db, String(released.changeset_id), {
      client: clientOf(writer),
      byId: writerId,
      items: [],
      versions: [ordinary.id],
    });

    const nestedPath = "Evidence/Left/AGENTS.md";
    const nested = resultOf<Written>(
      await writer.call("write", args({ path: nestedPath, content: "Nested evidence.\n" })),
      "create nested AGENTS"
    );
    expect(nested.changed).toBe(true);
    expect(writtenOf(nested, "create nested AGENTS").revision).toBe(1);
    await expectNewPage(
      db,
      {
        id: nested.node_id,
        notebook_id: notebook.id,
        parent_id: left.id,
        name: "AGENTS",
        created_at: String(nested.at),
      },
      writerId,
      clientOf(writer),
      "Nested evidence.\n"
    );
    await expectChangesetOf(db, String(nested.changeset_id), {
      client: clientOf(writer),
      byId: writerId,
      items: [nested.node_id],
      versions: [nested.node_id],
    });
    await refuse("edit", { path: nestedPath, old_string: "Nested", new_string: "Changed" }, "page.source_immutable");
    await refuse("move", { from: nestedPath, to: "Evidence/Left/Rules.md" });
    await move(
      admin,
      adminId,
      { id: nested.node_id, name: "AGENTS" },
      nestedPath,
      "Evidence/Left/Rules.md",
      left.id,
      "Rules"
    );
    await refuse("delete", { path: "Evidence/Left/Rules.md" });
    const bodies = await bodiesOf(db, notebook.id);
    const deleted = resultOf<Deleted>(
      await admin.call("delete", args({ path: "Evidence/Left/Rules.md" })),
      "delete nested source rules"
    );
    expect([deleted.node_id, deleted.pages, deleted.attachments]).toEqual([nested.node_id, 1, 0]);
    await expectSubtreeDeleted(db, nested.node_id, [nested.node_id], adminId);
    await expectChangesetOf(db, deleted.changeset_id, {
      client: clientOf(admin),
      byId: adminId,
      items: [nested.node_id],
      versions: [],
    });
    expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
  });
}

for (const role of ["writer", "admin"] as const) {
  test(`LW5 (API): a ${role}'s ordinary subtree containing a duplicate attachment stays entirely outside the source area`, async ({
    api,
    db,
    nervewiki,
  }, testInfo) => {
    const { pat, workspace } = await newTeam(api, testInfo);
    const writerEmail = emailFor(testInfo, "writer");
    const writer = await joinAs(api, pat, workspace.slug, writerEmail, "member");
    const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
    const known = await uploadAsset(api, writer, notebook.id, { name: "known.png", bytes: pngBytes }, raw.id);
    const folder = await createPage(api, writer, notebook.id, "Incoming");
    await createPage(api, writer, notebook.id, "Unique child", folder.id, "A unique source body\n");
    const copy = await uploadAsset(api, writer, notebook.id, { name: "renamed.png", bytes: pngBytes }, folder.id);
    const before = await pageStateOf(db, notebook.id);
    const files = storedBlobs(nervewiki.storageDir);
    const refused = await postMove(api, role === "writer" ? writer : pat, folder.id, { parent_id: raw.id });
    expect([refused.response.status, refused.error?.code, refused.error?.duplicate_of]).toEqual([
      409,
      "page.source_duplicate",
      known.id,
    ]);
    expect(await pageStateOf(db, notebook.id)).toEqual(before);
    expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
    // Remove only the conflicting ordinary attachment. The same writer/admin entry now succeeds and hashes the
    // unique page descendant; the previous refusal did not partially move it or consume a history revision.
    expect((await deleteNode(api, writer, copy.id)).response.status).toBe(204);
    await moveNode(api, role === "writer" ? writer : pat, folder.id, { parent_id: raw.id });
    expect((await placeOf(db, folder.id)).parentId).toBe(raw.id);
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  });
}

/** Node ids disambiguate the real root AGENTS and the nested namesake without stripping their spoken badges. */
function browserRow(page: BrowserPage, notebook: string, id: string) {
  return pageTree(page, notebook).locator(`a[href$="/pages/${id}"]`).locator("..");
}

async function menuOf(page: BrowserPage, notebook: string, id: string): Promise<string[]> {
  await browserRow(page, notebook, id)
    .getByRole("button", { name: /^Actions for / })
    .click();
  const items = await page.getByRole("menuitem").allTextContents();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menuitem")).toHaveCount(0);
  return items;
}

for (const role of ["writer", "admin"] as const) {
  test(`LW5 (page): ${role} menus distinguish source pages and attachments, nested AGENTS, and the editable protected roots`, async ({
    api,
    db,
    nervewiki,
    signedInPage,
  }, testInfo) => {
    const { tokens: adminTokens, adminId, pat, workspace } = await newOnboardedTeam(api, testInfo);
    const tokens =
      role === "writer"
        ? await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member")
        : adminTokens;
    const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
    const evidence = await createPage(api, pat, notebook.id, "Evidence", raw.id, "The original evidence.\n");
    const nested = await createPage(api, pat, notebook.id, "AGENTS", evidence.id, "Nested source rules.\n");
    const agents = (await listNodes(api, pat, notebook.id)).find(
      (node) => node.parent_id === null && node.name === "AGENTS"
    );
    if (!agents) throw new Error("the real root instructions are absent");
    const asset = await uploadAsset(api, pat, notebook.id, { name: "source.png", bytes: pngBytes }, evidence.id);
    await expectUploaded(db, nervewiki.storageDir, asset, pngBytes, adminId, "api");
    const before = await pageStateOf(db, notebook.id);
    const bodies = await bodiesOf(db, notebook.id);
    const page = await signedInPage(tokens);
    await page.goto(wikiPagePath(workspace.slug, notebook.id, nested.id));
    await expect(pageHeading(page, "AGENTS")).toBeVisible();
    await expect(page.getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
    await expect(browserRow(page, notebook.name, nested.id).getByRole("link")).toHaveAccessibleName("AGENTS (source)");
    expect(await menuOf(page, notebook.name, nested.id)).toEqual([
      "Paste Markdown",
      "Upload files",
      ...(role === "admin" ? ["Rename", "Move to…", "Delete"] : []),
    ]);
    expect(await menuOf(page, notebook.name, evidence.id)).toEqual([
      "Paste Markdown",
      "Upload files",
      ...(role === "admin" ? ["Rename", "Move to…", "Delete"] : []),
    ]);
    expect(await browserRow(page, notebook.name, evidence.id).evaluate((row) => (row as HTMLElement).draggable)).toBe(
      role === "admin"
    );
    expect(await menuOf(page, notebook.name, raw.id)).toEqual([
      "Paste Markdown",
      "Upload files",
      ...(role === "admin" ? ["Rename", "Move to…"] : []),
    ]);
    expect(await menuOf(page, notebook.name, agents.id)).toEqual(["New subpage"]);

    await browserRow(page, notebook.name, raw.id).getByRole("link").click();
    await expect(pageHeading(page, "raw")).toBeVisible();
    await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
    await browserRow(page, notebook.name, agents.id).getByRole("link").click();
    await expect(pageHeading(page, "AGENTS")).toBeVisible();
    await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
    await browserRow(page, notebook.name, evidence.id).getByRole("link").click();
    await expect(pageHeading(page, "Evidence")).toBeVisible();
    await expect(page.getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
    const section = page.getByRole("region", { name: "Attachments", exact: true });
    const assetRow = section.locator(`[data-attachment="${asset.id}"]`).locator("..");
    await expect(assetRow.getByRole("link")).toHaveAccessibleName("source.png (source) (opens in a new tab)");
    await expect(assetRow).toHaveAttribute("draggable", "true");
    await assetRow.getByRole("button", { name: "Actions for source.png", exact: true }).click();
    await expect(page.getByRole("menuitem")).toHaveText([
      "Open (opens in a new tab)",
      "Download",
      "Copy embed",
      ...(role === "admin" ? ["Rename", "Move to…", "Delete"] : []),
    ]);
    await page.keyboard.press("Escape");
    expect(await pageStateOf(db, notebook.id)).toEqual(before);
    expect(watchOf(page).apiRequests.filter((line) => /^(PATCH|DELETE) |^POST .*\/move$/.test(line))).toEqual([]);

    if (role === "admin") {
      await choosePageAction(page, notebook.name, "Evidence", "Rename");
      const rename = page.getByRole("dialog", { name: "Rename Evidence", exact: true });
      await rename.getByRole("textbox", { name: "Title", exact: true }).fill("Renamed evidence");
      const renamed = answerTo(page, "PATCH", `/api/v0/nodes/${evidence.id}`);
      await rename.getByRole("button", { name: "Save", exact: true }).click();
      expect((await renamed).status()).toBe(200);
      await expect(pageHeading(page, "Renamed evidence")).toBeVisible();
      await expectRenamed(db, evidence.id, "Evidence", "Renamed evidence", adminId);
      await expectChangesetOf(db, await latestManagement(db, evidence.id), {
        client: "web",
        byId: adminId,
        items: [evidence.id],
        versions: [],
      });
      const assetFrom = { ...(await placeOf(db, asset.id)), name: asset.name };
      await assetRow.getByRole("button", { name: "Actions for source.png", exact: true }).click();
      await page.getByRole("menuitem", { name: "Rename", exact: true }).click();
      const renameAsset = page.getByRole("dialog", { name: "Rename source.png", exact: true });
      await renameAsset.getByRole("textbox", { name: "Name", exact: true }).fill("renamed");
      const renamedAsset = answerTo(page, "PATCH", `/api/v0/nodes/${asset.id}`);
      await renameAsset.getByRole("button", { name: "Save", exact: true }).click();
      expect((await renamedAsset).status()).toBe(200);
      await expect(section.locator(`[data-attachment="${asset.id}"]`)).toHaveAccessibleName(
        "renamed.png (source) (opens in a new tab)"
      );
      await expectRelocated(db, asset.id, assetFrom, { parentId: evidence.id, name: "renamed.png" }, adminId);
      await expectChangesetOf(db, await latestManagement(db, asset.id), {
        client: "web",
        byId: adminId,
        items: [asset.id],
        versions: [],
      });
      expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
    }
    await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
  });
}

test("LW5 (page): a writer's Move and tree drag each wait for confirmation, cancel without a request, and enter sources only on acceptance", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newOnboardedTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const tokens = await joinOnboarded(api, pat, workspace.slug, writerEmail, "member");
  const writerId = await accountIdOf(db, writerEmail);
  const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
  const moved = await createPage(api, tokens.access_token, notebook.id, "Moved", null, "Move evidence.\n");
  const dragged = await createPage(api, tokens.access_token, notebook.id, "Dragged", null, "Dragged evidence.\n");
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, moved.id));
  await expect(pageHeading(page, "Moved")).toBeVisible();
  const confirmation = page.getByRole("alertdialog", { name: "Move into sources?", exact: true });
  const writes = (id: string) => watchOf(page).apiRequests.filter((line) => line === `POST /api/v0/nodes/${id}/move`);
  const before = await pageStateOf(db, notebook.id);
  const movedFrom = { ...(await placeOf(db, moved.id)), name: moved.name };
  const bodies = await bodiesOf(db, notebook.id);
  await choosePageAction(page, notebook.name, "Moved", "Move to…");
  const move = page.getByRole("dialog", { name: "Move Moved", exact: true });
  await move.getByLabel("Parent page", { exact: true }).selectOption(raw.id);
  await move.getByRole("button", { name: "Move", exact: true }).click();
  await expect(confirmation).toContainText("After moving, you cannot edit the content or manage these sources.");
  expect(writes(moved.id)).toEqual([]);
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  await confirmation.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(confirmation).toBeHidden();
  await expect(move).toBeVisible();
  expect(writes(moved.id)).toEqual([]);
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  await move.getByRole("button", { name: "Move", exact: true }).click();
  const moveAnswer = answerTo(page, "POST", `/api/v0/nodes/${moved.id}/move`);
  await confirmation.getByRole("button", { name: "Move", exact: true }).click();
  expect((await moveAnswer).status()).toBe(200);
  await expect(move).toBeHidden();
  expect(writes(moved.id)).toHaveLength(1);
  await expectRelocated(db, moved.id, movedFrom, { parentId: raw.id, name: moved.name }, writerId);
  await expectChangesetOf(db, await latestManagement(db, moved.id), {
    client: "web",
    byId: writerId,
    items: [moved.id],
    versions: [],
  });
  await expect(browserRow(page, notebook.name, moved.id).getByRole("link")).toHaveAccessibleName("Moved (source)");
  expect(await menuOf(page, notebook.name, moved.id)).toEqual(["Paste Markdown", "Upload files"]);
  await expect(page.getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);

  await browserRow(page, notebook.name, dragged.id).getByRole("link").click();
  await expect(pageHeading(page, "Dragged")).toBeVisible();
  const dragBefore = await pageStateOf(db, notebook.id);
  const draggedFrom = { ...(await placeOf(db, dragged.id)), name: dragged.name };
  await dragPage(page, notebook.name, "Dragged", "raw (source area)", "into");
  await expect(confirmation).toBeVisible();
  expect(writes(dragged.id)).toEqual([]);
  await confirmation.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(confirmation).toBeHidden();
  expect(writes(dragged.id)).toEqual([]);
  expect(await pageStateOf(db, notebook.id)).toEqual(dragBefore);
  await dragPage(page, notebook.name, "Dragged", "raw (source area)", "into");
  const dragAnswer = answerTo(page, "POST", `/api/v0/nodes/${dragged.id}/move`);
  await confirmation.getByRole("button", { name: "Move", exact: true }).click();
  expect((await dragAnswer).status()).toBe(200);
  await expect(confirmation).toBeHidden();
  expect(writes(dragged.id)).toHaveLength(1);
  await expectRelocated(db, dragged.id, draggedFrom, { parentId: raw.id, name: dragged.name }, writerId);
  await expectChangesetOf(db, await latestManagement(db, dragged.id), {
    client: "web",
    byId: writerId,
    items: [dragged.id],
    versions: [],
  });
  await expect(browserRow(page, notebook.name, dragged.id).getByRole("link")).toHaveAccessibleName("Dragged (source)");
  expect(await menuOf(page, notebook.name, dragged.id)).toEqual(["Paste Markdown", "Upload files"]);
  await expect(page.getByRole("button", { name: "Edit", exact: true })).toHaveCount(0);
  expect(await bodiesOf(db, notebook.id)).toEqual(bodies);
  await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
});

test("LW5 (page): page move, tree drag and attachment move reject a duplicate atomically and its path focuses the real source attachment", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { pat, workspace } = await newOnboardedTeam(api, testInfo);
  const tokens = await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member");
  const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
  const archive = await createPage(api, pat, notebook.id, "Archive", raw.id);
  const known = await uploadAsset(api, pat, notebook.id, { name: "known.png", bytes: pngBytes }, archive.id);
  const incoming = await createPage(api, tokens.access_token, notebook.id, "Incoming");
  await createPage(
    api,
    tokens.access_token,
    notebook.id,
    "Unique child",
    incoming.id,
    "The unique child remains outside.\n"
  );
  const incomingAsset = await uploadAsset(
    api,
    tokens.access_token,
    notebook.id,
    { name: "different-name.png", bytes: pngBytes },
    incoming.id
  );
  const before = await pageStateOf(db, notebook.id);
  const files = storedBlobs(nervewiki.storageDir);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, incoming.id));
  await expect(pageHeading(page, "Incoming")).toBeVisible();
  await choosePageAction(page, notebook.name, "Incoming", "Move to…");
  const dialog = page.getByRole("dialog", { name: "Move Incoming", exact: true });
  await dialog.getByLabel("Parent page", { exact: true }).selectOption(raw.id);
  await dialog.getByRole("button", { name: "Move", exact: true }).click();
  const confirmation = page.getByRole("alertdialog", { name: "Move into sources?", exact: true });
  watchOf(page).expectConsole({ errors: [failedToLoad(409)] });
  const rejected = answerTo(page, "POST", `/api/v0/nodes/${incoming.id}/move`);
  await confirmation.getByRole("button", { name: "Move", exact: true }).click();
  const response = await rejected;
  expect(response.status()).toBe(409);
  expect(await response.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  const target = dialog.getByRole("link", { name: "raw/Archive/known.png", exact: true });
  await expect(target).toBeVisible();
  await expect(dialog).not.toContainText(known.id);
  await target.click();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, archive.id));
  await expect(pageHeading(page, "Archive")).toBeVisible();
  const found = page
    .getByRole("region", { name: "Attachments", exact: true })
    .locator(`[data-attachment="${known.id}"]`);
  await expect(found).toHaveAccessibleName("known.png (source) (opens in a new tab)");
  await expect(found).toBeFocused();
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);

  // The same refused subtree also goes through the real tree monitor's Alert consumer.
  await browserRow(page, notebook.name, incoming.id).getByRole("link").click();
  await expect(pageHeading(page, "Incoming")).toBeVisible();
  await dragPage(page, notebook.name, "Incoming", "raw (source area)", "into");
  await expect(confirmation).toBeVisible();
  watchOf(page).expectConsole({ errors: [failedToLoad(409)] });
  const dragRejected = answerTo(page, "POST", `/api/v0/nodes/${incoming.id}/move`);
  await confirmation.getByRole("button", { name: "Move", exact: true }).click();
  const dragResponse = await dragRejected;
  expect(dragResponse.status()).toBe(409);
  expect(await dragResponse.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  const treeAlert = pageTree(page, notebook.name).getByRole("alert");
  const treeTarget = treeAlert.getByRole("link", { name: "raw/Archive/known.png", exact: true });
  await expect(treeTarget).toBeVisible();
  await expect(treeAlert).not.toContainText(known.id);
  await treeTarget.click();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, archive.id));
  await expect(pageHeading(page, "Archive")).toBeVisible();
  await expect(found).toHaveAccessibleName("known.png (source) (opens in a new tab)");
  await expect(found).toBeFocused();
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);

  // Moving the attachment itself has its own form and close/focus lifecycle, with the same real refusal.
  await browserRow(page, notebook.name, incoming.id).getByRole("link").click();
  await expect(pageHeading(page, "Incoming")).toBeVisible();
  const incomingSection = page.getByRole("region", { name: "Attachments", exact: true });
  await incomingSection.getByRole("button", { name: "Actions for different-name.png", exact: true }).click();
  await page.getByRole("menuitem", { name: "Move to…", exact: true }).click();
  const assetDialog = page.getByRole("dialog", { name: "Move different-name.png", exact: true });
  await assetDialog.getByLabel("Parent page", { exact: true }).selectOption(raw.id);
  await assetDialog.getByRole("button", { name: "Move", exact: true }).click();
  await expect(confirmation).toBeVisible();
  watchOf(page).expectConsole({ errors: [failedToLoad(409)] });
  const assetRejected = answerTo(page, "POST", `/api/v0/nodes/${incomingAsset.id}/move`);
  await confirmation.getByRole("button", { name: "Move", exact: true }).click();
  const assetResponse = await assetRejected;
  expect(assetResponse.status()).toBe(409);
  expect(await assetResponse.json()).toMatchObject({ code: "page.source_duplicate", duplicate_of: known.id });
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  const assetTarget = assetDialog.getByRole("link", { name: "raw/Archive/known.png", exact: true });
  await expect(assetTarget).toBeVisible();
  await expect(assetDialog).not.toContainText(known.id);
  await assetTarget.click();
  await expect(assetDialog).toBeHidden();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, archive.id));
  await expect(pageHeading(page, "Archive")).toBeVisible();
  await expect(found).toHaveAccessibleName("known.png (source) (opens in a new tab)");
  await expect(found).toBeFocused();
  expect(await pageStateOf(db, notebook.id)).toEqual(before);
  expect(storedBlobs(nervewiki.storageDir)).toEqual(files);
  expect(watchOf(page).apiRequests.filter((line) => line === `POST /api/v0/nodes/${incoming.id}/move`)).toHaveLength(2);
  expect(
    watchOf(page).apiRequests.filter((line) => line === `POST /api/v0/nodes/${incomingAsset.id}/move`)
  ).toHaveLength(1);
  await expectSourceHashes(db, nervewiki.storageDir, notebook.id);
});
