import type { ApiClient } from "@nervewiki/api-client";
import type { TestInfo } from "@playwright/test";

import { expectAliveSessions } from "../../fixtures/assert/collab";
import { accountIdOf } from "../../fixtures/assert/identity";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import {
  expectChangesetOf,
  expectContentWritten,
  expectRevert,
  expectSessionGone,
  expectVersions,
  pageStateOf,
  type StoredVersion,
} from "../../fixtures/assert/page";
import { createToken, emailFor } from "../../fixtures/auth";
import type { Database } from "../../fixtures/db";
import { listChangesets, revertChangeset } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, readPage, resultOf, writtenOf, type McpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import {
  createPage,
  endSession,
  getPage,
  getVersions,
  listNodes,
  moveNode,
  openSession,
  postRestore,
  postSession,
  putContent,
  readContent,
  renameNode,
  toggleTask,
  writeContent,
} from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// LW4 (M10/P2 section 5): sources are written once, even by an admin;
// system link rewrites and their reverts remain possible. Every refused
// public write compares the whole small notebook's durable state.
// MCP has no task, session, version-restore or undo tool: REST covers the
// first three; the MCP rewrite story explicitly bridges its undos to REST.

const evidence = "- [ ] evidence\n";
const linked = "\ufeff- [ ] evidence\r\nSee [[Target]] and [md](/Target.md).\r\n";
const rewritten = "\ufeff- [ ] evidence\r\nSee [[Renamed]] and [md](/Renamed.md).\r\n";

/** The common fields of write/edit's real structured answers. */
interface Written {
  path: string;
  node_id: string;
  version: string;
  changed: boolean;
  changeset_id: string | null;
  at: string | null;
}

/** move's structured answer. */
interface Moved {
  path: string;
  node_id: string;
  changed: boolean;
  changeset_id: string | null;
}

async function sourceTeam(api: ApiClient, db: Database, testInfo: TestInfo) {
  const team = await newTeam(api, testInfo);
  const writerEmail = emailFor(testInfo, "writer");
  const writerPat = await joinAs(api, team.pat, team.workspace.slug, writerEmail, "member");
  return { ...team, writerPat, writerId: await accountIdOf(db, writerEmail) };
}

/** Resolve the source root by its notebook identity, not by its title. */
async function sourceWiki(api: ApiClient, pat: string, slug: string) {
  const notebook = await createNotebook(api, pat, slug, "Sources", "editor", {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const nodes = await listNodes(api, pat, notebook.id);
  const raw = nodes.find((node) => node.id === notebook.source_root_id);
  const agents = nodes.find((node) => node.parent_id === null && node.name === "AGENTS");
  if (!raw || !agents) throw new Error(`the LLM Wiki has no source root or AGENTS: ${JSON.stringify(nodes)}`);
  return { notebook, raw, agents };
}

/** Two source ancestors keep a direct-parent-only check from passing. */
async function deepFolder(api: ApiClient, pat: string, notebookId: string, rawId: string) {
  const folder = await createPage(api, pat, notebookId, "Folder", rawId);
  return createPage(api, pat, notebookId, "Deep", folder.id);
}

async function expectAPIImmutable(
  db: Database,
  notebookId: string,
  what: string,
  call: () => Promise<{ response: Response; error?: { code: string } }>
) {
  const before = await pageStateOf(db, notebookId);
  const answer = await call();
  expect([answer.response.status, answer.error?.code], what).toEqual([409, "page.source_immutable"]);
  expect(await pageStateOf(db, notebookId), `${what} leaves durable state unchanged`).toEqual(before);
}

async function expectMCPRefusal(
  db: Database,
  notebookId: string,
  agent: McpClient,
  tool: string,
  args: Record<string, unknown>,
  code = "page.source_immutable"
) {
  const before = await pageStateOf(db, notebookId);
  const error = errorOf(await agent.call(tool, args), `${tool} ${JSON.stringify(args)}`);
  expect(error.code).toBe(code);
  expect(await pageStateOf(db, notebookId), `${tool} leaves durable state unchanged`).toEqual(before);
  return error;
}

test("LW4 (API): writers create deep sources, but writers and admins cannot change their content, tasks, versions or open sessions; empty sources allow one first write and same-byte writes are no-ops", async ({
  api,
  db,
}, testInfo) => {
  const { pat, writerPat, writerId, workspace } = await sourceTeam(api, db, testInfo);
  const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
  const deep = await deepFolder(api, writerPat, notebook.id, raw.id);
  const source = await createPage(api, writerPat, notebook.id, "Source", deep.id, evidence);
  await expectContentWritten(db, source, evidence, writerId);
  const empty = await createPage(api, writerPat, notebook.id, "Empty", deep.id);

  // Both versions are legitimate public writes, made before History entered the source area.
  const history = await createPage(api, writerPat, notebook.id, "History", null, "old evidence\n");
  await writeContent(api, writerPat, history.id, { content: "current evidence\n", base_revision: 1 });
  await moveNode(api, pat, history.id, { parent_id: deep.id });
  const refuseAs = async (credential: string, role: string) => {
    await expectAPIImmutable(db, notebook.id, `${role} PUT`, () =>
      putContent(api, credential, source.id, { content: "changed\n", base_revision: 1 })
    );
    await expectAPIImmutable(db, notebook.id, `${role} task`, () =>
      toggleTask(api, credential, source.id, { base_revision: 1, offset: 3, checked: true })
    );
    await expectAPIImmutable(db, notebook.id, `${role} changed version`, () =>
      postRestore(api, credential, history.id, 1, 2)
    );
    await expectAPIImmutable(db, notebook.id, `${role} same-byte version`, () =>
      postRestore(api, credential, history.id, 2, 2)
    );
    await expectAPIImmutable(db, notebook.id, `${role} empty version`, () =>
      postRestore(api, credential, empty.id, 1, 1)
    );
    await expectAPIImmutable(db, notebook.id, `${role} source session`, () => postSession(api, credential, source.id));
    await expectAPIImmutable(db, notebook.id, `${role} empty session`, () => postSession(api, credential, empty.id));
    const before = await pageStateOf(db, notebook.id);
    const same = await writeContent(api, credential, source.id, { content: evidence, base_revision: 1 });
    expect(same.revision).toBe(1);
    expect(await pageStateOf(db, notebook.id), `${role} one-shot same-byte write`).toEqual(before);
  };
  await refuseAs(writerPat, "writer");
  await refuseAs(pat, "admin");

  const beforeFirst = await pageStateOf(db, notebook.id);
  const first = await writeContent(api, writerPat, empty.id, { content: "first evidence\n", base_revision: 1 });
  expect(first.revision).toBe(2);
  await expectContentWritten(db, first, "first evidence\n", writerId);
  expect(await pageStateOf(db, notebook.id), "the snapshot observes an allowed first write").not.toEqual(beforeFirst);
  await expectAPIImmutable(db, notebook.id, "second source write", () =>
    putContent(api, writerPat, empty.id, { content: "second evidence\n", base_revision: 2 })
  );
  await expectVersions(db, source.id, [
    { base: null, revision: 1, content: evidence, kind: "edit", byId: writerId, client: "api" },
  ]);
  await expectVersions(db, empty.id, [
    { base: null, revision: 1, content: "", kind: "edit", byId: writerId, client: "api" },
    { base: 1, revision: 2, content: "first evidence\n", kind: "edit", byId: writerId, client: "api" },
  ]);
});

test("LW4 (API): valid sessions opened outside the source area cannot save changed or identical bytes after an admin moves either an empty or nonempty page inside", async ({
  api,
  db,
}, testInfo) => {
  const { pat, writerPat, workspace } = await sourceTeam(api, db, testInfo);
  const { notebook, raw } = await sourceWiki(api, pat, workspace.slug);
  const deep = await deepFolder(api, writerPat, notebook.id, raw.id);
  const oldSession = async (credential: string, title: string, content: string) => {
    const page = await createPage(api, credential, notebook.id, title, null, content);
    const session = await openSession(api, credential, page.id);
    await expectAliveSessions(db, page.id, [session.id]);
    await moveNode(api, pat, page.id, { parent_id: deep.id });
    await expectAPIImmutable(db, notebook.id, `${title} changed session save`, () =>
      putContent(api, credential, page.id, {
        content: "changed through the old session\n",
        base_revision: 1,
        edit_session_id: session.id,
      })
    );
    await expectAPIImmutable(db, notebook.id, `${title} same-byte session save`, () =>
      putContent(api, credential, page.id, { content, base_revision: 1, edit_session_id: session.id })
    );
    await expectAPIImmutable(db, notebook.id, `${title} reopening`, () => postSession(api, credential, page.id));
    await expectAliveSessions(db, page.id, [session.id]);
    expect(await readContent(api, credential, page.id)).toMatchObject({ content, revision: 1 });
    expect((await endSession(api, credential, session.id)).response.status).toBe(204);
    await expectSessionGone(db, session.id);
  };
  await oldSession(writerPat, "Writer empty", "");
  await oldSession(writerPat, "Writer nonempty", evidence);
  await oldSession(pat, "Admin empty", "");
  await oldSession(pat, "Admin nonempty", "admin evidence\n");
});

test("LW4 (API): source-root instructions, root AGENTS, ordinary wiki pages and the same tree in standard mode keep PUT, task, version restore and session writes", async ({
  api,
  db,
}, testInfo) => {
  const { pat, writerPat, writerId, workspace } = await sourceTeam(api, db, testInfo);
  const { notebook, raw, agents } = await sourceWiki(api, pat, workspace.slug);
  const ordinary = await createPage(api, writerPat, notebook.id, "Ordinary", null, "ordinary\n");
  const standard = await createNotebook(api, pat, workspace.slug, "Standard", "editor", { mode: "standard" });
  const standardRaw = await createPage(api, writerPat, standard.id, "raw");
  const deep = await deepFolder(api, writerPat, standard.id, standardRaw.id);
  const sameShape = await createPage(api, writerPat, standard.id, "Source", deep.id, "standard evidence\n");
  const writable = async (id: string) => {
    const original = await readContent(api, writerPat, id);
    const put = await writeContent(api, writerPat, id, { content: evidence, base_revision: original.revision });
    await expectContentWritten(db, put, evidence, writerId);
    const task = await toggleTask(api, writerPat, id, { base_revision: put.revision, offset: 3, checked: true });
    expect(task.response.status, JSON.stringify(task.error)).toBe(200);
    expect(task.data?.revision).toBe(put.revision + 1);
    expect(await readContent(api, writerPat, id)).toMatchObject({
      content: "- [x] evidence\n",
      revision: put.revision + 1,
    });
    const restore = await postRestore(api, writerPat, id, original.revision, put.revision + 1);
    expect(restore.response.status, JSON.stringify(restore.error)).toBe(200);
    expect(await readContent(api, writerPat, id)).toMatchObject({
      content: original.content,
      revision: original.revision + 3,
    });
    const session = await openSession(api, writerPat, id);
    const saved = await writeContent(api, writerPat, id, {
      content: "saved in an ordinary session\n",
      base_revision: original.revision + 3,
      edit_session_id: session.id,
    });
    expect(saved.revision).toBe(original.revision + 4);
    await expectContentWritten(db, saved, "saved in an ordinary session\n", writerId);
    expect((await endSession(api, writerPat, session.id)).response.status).toBe(204);
    await expectSessionGone(db, session.id);
  };
  await writable(raw.id);
  await writable(agents.id);
  await writable(ordinary.id);
  await writable(sameShape.id);
});

/** A real rename and both real undos preserve bytes, the link index and the stored rewrite markers. */
async function rewriteChain(
  api: ApiClient,
  db: Database,
  credential: string,
  byId: string,
  sourceId: string,
  targetId: string,
  firstClient: string,
  rename: () => Promise<string>,
  readBack?: (revision: number, content: string) => Promise<void>
) {
  const versions: StoredVersion[] = [
    { base: null, revision: 1, content: linked, kind: "edit", byId, client: firstClient, rewrite: false },
  ];
  const check = async (revision: number, content: string, target: string) => {
    expect(await readContent(api, credential, sourceId)).toMatchObject({ content, revision });
    const targetPage = await getPage(api, credential, targetId);
    expect(targetPage.response.status, JSON.stringify(targetPage.error)).toBe(200);
    expect(targetPage.data).toMatchObject({ name: target, parent_id: null, revision: 1 });
    await expectIndexedLinks(db, sourceId, [
      { kind: "wikilink", property: null, target, resolved: targetId },
      { kind: "link", property: null, target: `/${target}.md`, resolved: targetId },
    ]);
    await expectVersions(db, sourceId, versions);
    if (readBack) await readBack(revision, content);
  };
  await check(1, linked, "Target");
  const renamed = await rename();
  await expectChangesetOf(db, renamed, { client: firstClient, byId, items: [targetId], versions: [sourceId] });
  versions.push({ base: 1, revision: 2, content: rewritten, kind: "edit", byId, client: firstClient, rewrite: true });
  await check(2, rewritten, "Renamed");

  // MCP exposes history reads, not undo. When called from an MCP story these two writes are an explicit API bridge.
  const undo = await revertChangeset(api, credential, renamed);
  expect(undo.response.status, JSON.stringify(undo.error)).toBe(200);
  expect(undo.data?.changed).toBe(true);
  const undoId = undo.data?.changeset_id;
  if (!undoId) throw new Error("the rename's undo answered without a changeset");
  versions.push({
    base: 2,
    revision: 3,
    content: linked,
    kind: "revert",
    byId,
    client: "api",
    revertsId: renamed,
    rewrite: true,
  });
  await expectRevert(db, undoId, byId, "api", { revertsId: renamed, items: 1, versions: 1 });
  await check(3, linked, "Target");
  const again = await revertChangeset(api, credential, undoId);
  expect(again.response.status, JSON.stringify(again.error)).toBe(200);
  expect(again.data?.changed).toBe(true);
  const againId = again.data?.changeset_id;
  if (!againId) throw new Error("the undo's undo answered without a changeset");
  expect(new Set([renamed, undoId, againId]).size).toBe(3);
  versions.push({
    base: 3,
    revision: 4,
    content: rewritten,
    kind: "revert",
    byId,
    client: "api",
    revertsId: undoId,
    rewrite: true,
  });
  await expectRevert(db, againId, byId, "api", { revertsId: undoId, items: 1, versions: 1 });
  await check(4, rewritten, "Renamed");
}

test("LW4 (API): renaming an ordinary target rewrites a source byte for byte, and undo then undo-of-undo preserve links and all three system rewrite versions", async ({
  api,
  db,
}, testInfo) => {
  const { pat, writerPat, writerId, workspace } = await sourceTeam(api, db, testInfo);
  const { notebook, raw, agents } = await sourceWiki(api, pat, workspace.slug);
  const deep = await deepFolder(api, writerPat, notebook.id, raw.id);
  const target = await createPage(api, writerPat, notebook.id, "Target");
  const source = await createPage(api, writerPat, notebook.id, "Source", deep.id, linked);
  const rootBefore = await readContent(api, pat, raw.id);
  const agentsBefore = await readContent(api, pat, agents.id);
  await rewriteChain(api, db, writerPat, writerId, source.id, target.id, "api", async () => {
    const renamed = await renameNode(api, writerPat, target.id, "Renamed");
    expect(renamed.response.status, JSON.stringify(renamed.error)).toBe(200);
    const log = await listChangesets(api, writerPat, notebook.id, { pageId: target.id, limit: 1 });
    expect(log.response.status, JSON.stringify(log.error)).toBe(200);
    const change = log.data?.data[0];
    expect(change).toMatchObject({ kind: "edit", node_count: 1, content_count: 1 });
    expect(change?.changes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ node: expect.objectContaining({ id: target.id }), kind: "renamed" }),
      ])
    );
    if (!change) throw new Error("the target's rename has no changeset");
    return change.id;
  });
  expect(await readContent(api, pat, raw.id)).toEqual(rootBefore);
  expect(await readContent(api, pat, agents.id)).toEqual(agentsBefore);
  await expectVersions(db, target.id, [
    { base: null, revision: 1, content: "", kind: "edit", byId: writerId, client: "api" },
  ]);
});

for (const scope of ["notebook", "account"] as const) {
  test(`LW4 (MCP): ${scope} scope source write, edit and batch are immutable for writer and admin; ordinary writes remain possible and target rename rewrites survive API undo and undo-of-undo read back through MCP`, async ({
    api,
    db,
    nervewiki,
  }, testInfo) => {
    const { pat, writerPat, writerId, workspace } = await sourceTeam(api, db, testInfo);
    const { notebook, raw, agents } = await sourceWiki(api, pat, workspace.slug);
    const deep = await deepFolder(api, writerPat, notebook.id, raw.id);
    const target = await createPage(api, writerPat, notebook.id, "Target");
    const empty = await createPage(api, writerPat, notebook.id, "Empty", deep.id);
    const writerClient = `lw4-${scope}-writer`;
    const adminClient = `lw4-${scope}-admin`;
    const writerToken = await createToken(api, writerPat, { name: writerClient });
    const adminToken = await createToken(api, pat, { name: adminClient });
    const entry = scope === "notebook" ? { notebook: notebook.id } : {};
    const agent = await mcpClient(nervewiki.baseURL, writerToken.token, { ...entry, client: writerClient });
    const admin = await mcpClient(nervewiki.baseURL, adminToken.token, { ...entry, client: adminClient });
    const args = (value: Record<string, unknown>) =>
      scope === "account" ? { ...value, notebook: notebook.id } : value;
    const path = "raw/Folder/Deep/Source.md";
    const first = resultOf<Written>(await agent.call("write", args({ path, content: linked })), "create a source");
    expect([first.path, first.version, first.changed]).toEqual([path, `${first.node_id}:1`, true]);
    await expectContentWritten(db, writtenOf(first, "create a source"), linked, writerId);
    await expectChangesetOf(db, String(first.changeset_id), {
      client: `mcp:${writerClient}`,
      byId: writerId,
      items: [first.node_id],
      versions: [first.node_id],
    });
    const read = await readPage(agent, args({ path }));
    expect([read.text, read.meta.version]).toEqual([linked, first.version]);

    const refuseAs = async (caller: McpClient) => {
      await expectMCPRefusal(
        db,
        notebook.id,
        caller,
        "write",
        args({
          path,
          content: "changed\n",
          base_version: read.meta.version,
        })
      );
      await expectMCPRefusal(
        db,
        notebook.id,
        caller,
        "edit",
        args({
          path,
          old_string: "evidence",
          new_string: "changed",
          base_version: read.meta.version,
        })
      );
      const batch = await expectMCPRefusal(
        db,
        notebook.id,
        caller,
        "batch",
        args({
          message: "A source refusal rolls the ordinary creation back",
          ops: [
            { op: "write", path: "Rolled back.md", content: "must not persist\n" },
            { op: "edit", path, old_string: "evidence", new_string: "changed", base_version: read.meta.version },
          ],
        })
      );
      expect(batch.op).toBe(1);
      expect(batch.message).toContain("ops[1] (edit)");
      const batchWrite = await expectMCPRefusal(
        db,
        notebook.id,
        caller,
        "batch",
        args({
          message: "A source overwrite rolls the ordinary creation back",
          ops: [
            { op: "write", path: "Rolled back.md", content: "must not persist\n" },
            { op: "write", path, content: "changed\n", base_version: read.meta.version },
          ],
        })
      );
      expect(batchWrite.op).toBe(1);
      expect(batchWrite.message).toContain("ops[1] (write)");
    };
    await refuseAs(agent);
    await refuseAs(admin);
    const taken = await expectMCPRefusal(
      db,
      notebook.id,
      agent,
      "write",
      args({ path, content: "changed\n" }),
      "page.title_taken"
    );
    expect(taken.message).toContain("base_version");
    const missing = await expectMCPRefusal(
      db,
      notebook.id,
      agent,
      "edit",
      args({
        path,
        old_string: "absent text",
        new_string: "changed",
        base_version: read.meta.version,
      }),
      "validation_failed"
    );
    expect(missing.fields).toEqual(
      expect.arrayContaining([expect.objectContaining({ field: "old_string", code: "not_allowed" })])
    );
    const beforeSame = await pageStateOf(db, notebook.id);
    const same = resultOf<Written>(
      await agent.call("write", args({ path, content: linked })),
      "same-byte source write"
    );
    expect([same.changed, same.version, same.changeset_id, same.at]).toEqual([false, first.version, null, null]);
    expect(await pageStateOf(db, notebook.id)).toEqual(beforeSame);

    const emptyPath = "raw/Folder/Deep/Empty.md";
    const filled = resultOf<Written>(
      await agent.call(
        "write",
        args({
          path: emptyPath,
          content: "first evidence\n",
          base_version: `${empty.id}:${empty.revision}`,
        })
      ),
      "first write to an empty source"
    );
    expect([filled.changed, filled.version]).toEqual([true, `${empty.id}:2`]);
    await expectContentWritten(db, writtenOf(filled, "first source content"), "first evidence\n", writerId);
    await expectMCPRefusal(
      db,
      notebook.id,
      agent,
      "write",
      args({
        path: emptyPath,
        content: "second evidence\n",
        base_version: filled.version,
      })
    );
    await expectVersions(db, empty.id, [
      { base: null, revision: 1, content: "", kind: "edit", byId: writerId, client: "api" },
      {
        base: 1,
        revision: 2,
        content: "first evidence\n",
        kind: "edit",
        byId: writerId,
        client: `mcp:${writerClient}`,
      },
    ]);

    // The strict root and ordinary paths still accept both real MCP write and edit calls.
    const writable = async (pagePath: string, id?: string) => {
      const existing = id === undefined ? undefined : await readPage(agent, args({ path: pagePath }));
      const put = resultOf<Written>(
        await agent.call(
          "write",
          args({
            path: pagePath,
            content: "editable instructions\n",
            ...(existing ? { base_version: existing.meta.version } : {}),
          })
        ),
        `write ${pagePath}`
      );
      await expectContentWritten(db, writtenOf(put, `write ${pagePath}`), "editable instructions\n", writerId);
      await expectChangesetOf(db, String(put.changeset_id), {
        client: `mcp:${writerClient}`,
        byId: writerId,
        items: id === undefined ? [put.node_id] : [],
        versions: [put.node_id],
      });
      const edit = resultOf<Written>(
        await agent.call(
          "edit",
          args({
            path: pagePath,
            old_string: "editable",
            new_string: "updated",
            base_version: put.version,
          })
        ),
        `edit ${pagePath}`
      );
      await expectContentWritten(db, writtenOf(edit, `edit ${pagePath}`), "updated instructions\n", writerId);
      await expectChangesetOf(db, String(edit.changeset_id), {
        client: `mcp:${writerClient}`,
        byId: writerId,
        items: [],
        versions: [put.node_id],
      });
    };
    await writable("raw.md", raw.id);
    await writable("AGENTS.md", agents.id);
    await writable("Ordinary.md");
    const rootBefore = await readContent(api, pat, raw.id);
    const agentsBefore = await readContent(api, pat, agents.id);
    await rewriteChain(
      api,
      db,
      writerPat,
      writerId,
      first.node_id,
      target.id,
      `mcp:${writerClient}`,
      async () => {
        const renamed = resultOf<Moved>(
          await agent.call(
            "move",
            args({
              from: "Target.md",
              to: "Renamed.md",
              update_links: true,
            })
          ),
          "rename the ordinary target through MCP"
        );
        expect([renamed.path, renamed.node_id, renamed.changed]).toEqual(["Renamed.md", target.id, true]);
        if (!renamed.changeset_id) throw new Error("MCP move answered without its changeset");
        return renamed.changeset_id;
      },
      async (revision, content) => {
        const current = await readPage(agent, args({ path }));
        expect([current.meta.version, current.text]).toEqual([`${first.node_id}:${revision}`, content]);
        const versions = resultOf<{ current_version: string; versions: unknown[]; next_cursor: string | null }>(
          await agent.call("versions", args({ path })),
          "read the source's versions through MCP"
        );
        const rest = await getVersions(api, writerPat, first.node_id);
        expect(rest.response.status, JSON.stringify(rest.error)).toBe(200);
        expect([versions.current_version, versions.next_cursor, versions.versions]).toEqual([
          current.meta.version,
          null,
          rest.data?.data,
        ]);
        expect(versions.versions).toHaveLength(revision);
      }
    );
    expect(await readContent(api, pat, raw.id)).toEqual(rootBefore);
    expect(await readContent(api, pat, agents.id)).toEqual(agentsBefore);
  });
}
