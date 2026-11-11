import { createHash } from "node:crypto";

import type { ApiClient, Page } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectUpgrade } from "../../fixtures/assert/wiki";
import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { emailFor } from "../../fixtures/auth";
import type { Database } from "../../fixtures/db";
import { revertChangeset } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, type McpClient, mcpClient, resultOf } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { postRevert } from "../../fixtures/transfer";
import { approveUpgrade, getUpgrade, postReject, startUpgrade, submitUpgrade } from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP7, an upgrade approved (M11/P3 design 5): a second admin approves the
// submitted upgrade of a notebook whose index at the root was deleted
// before it started, with a page and an attachment in raw and a note the
// agent gives its frontmatter. The notebook is an LLM Wiki whose source
// root is raw, its sources hashed as the SHA-256 of the bytes the test
// wrote, the upgrade's baseline gone; the
// deletion's revert, which would bring index back at the root, is
// page.virtual_file, in the request and in the background (decision 181).
// Through MCP, the agent lists the LLM Wiki's virtual index.md and
// log.md, and AGENTS is protected. The page version is S3's.

/** The story's notebook, submitted: its index deleted before, raw holding Paper and chart.png, Note at the root. */
async function submitted(
  api: ApiClient,
  db: Database,
  testInfo: Parameters<typeof newTeam>[1],
  baseURL: string,
  viaAgent: boolean
) {
  const { pat, workspace } = await newTeam(api, testInfo);
  const danaEmail = emailFor(testInfo, "dana");
  const dana = await joinAs(api, pat, workspace.slug, danaEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, danaEmail), "admin");
  const index = await createPage(api, pat, notebook.id, "index", null, "# Index\n");
  const gone = await deleteNode(api, pat, index.id);
  expect(gone.response.status).toBe(204);
  const deleted = await latestChangeset(db, notebook.id);
  const raw = await createPage(api, pat, notebook.id, "raw");
  await createPage(api, pat, notebook.id, "Paper", raw.id, "The paper.\n");
  await uploadAsset(api, pat, notebook.id, { name: "chart.png", bytes: pngBytes }, raw.id);
  const note = await createPage(api, pat, notebook.id, "Note", null, "# Note\n");
  await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(baseURL, pat, { notebook: notebook.id });
  await frontmatter(agent, note);
  if (viaAgent) {
    expect(resultOf<{ state: string }>(await agent.call("upgrade_submit", {}), "the agent's submission").state).toBe(
      "reviewing"
    );
  } else {
    await submitUpgrade(api, pat, notebook.id);
  }
  return { pat, dana, notebook, raw, deleted, agent };
}

/** The agent gives Note its type and summary above its body. */
async function frontmatter(agent: McpClient, note: Page): Promise<void> {
  resultOf(
    await agent.call("write", {
      path: "Note.md",
      content: "---\ntype: note\nsummary: A note.\n---\n# Note\n",
      base_version: `${note.id}:${note.revision}`,
    }),
    "the agent's frontmatter of Note"
  );
}

/** The newest changeset of the notebook notebookId. */
async function latestChangeset(db: Database, notebookId: string): Promise<string> {
  const rows = await db.query<{ id: string }>(
    "SELECT id FROM changesets WHERE notebook_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1",
    [notebookId]
  );
  return rows[0]?.id ?? "";
}

/** The notebook notebookId's mode, its source root and its upgrade under way. */
async function modeOf(db: Database, notebookId: string) {
  return db.query("SELECT mode, source_root_id, upgrade_id FROM notebooks WHERE id = $1", [notebookId]);
}

/** The notebook notebookId's nodes with a source hash, each its name and the hash in hex. */
async function hashed(db: Database, notebookId: string) {
  return db.query(
    `SELECT name, encode(source_hash, 'hex') AS hash FROM nodes
      WHERE notebook_id = $1 AND source_hash IS NOT NULL ORDER BY name`,
    [notebookId]
  );
}

/** The SHA-256 of bytes, in hex. */
function sha256(bytes: string | Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex");
}

test("UP7 (API): a second admin approves the submitted upgrade; the notebook is an LLM Wiki whose source root is raw, its page and attachment hashed as the SHA-256 of the bytes written, the baseline gone and the upgrade read no more; reverting the deletion of index at the root is page.virtual_file, in the request and in the background", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, dana, notebook, raw, deleted } = await submitted(api, db, testInfo, nervewiki.baseURL, false);
  const before = await getUpgrade(api, pat, notebook.id);
  expect(before.data?.state).toBe("reviewing");

  const approved = await approveUpgrade(api, dana, notebook.id);
  expect(approved).toEqual({ ...before.data, state: "approved" });
  await expectUpgrade(db, approved);
  expect(await modeOf(db, notebook.id)).toEqual([{ mode: "llm_wiki", source_root_id: raw.id, upgrade_id: null }]);
  expect(await hashed(db, notebook.id)).toEqual([
    { name: "Paper", hash: sha256("The paper.\n") },
    { name: "chart.png", hash: sha256(pngBytes) },
  ]);
  expect(
    await db.query("SELECT count(*)::int AS rows FROM upgrade_nodes WHERE upgrade_id = $1", [approved.id])
  ).toEqual([{ rows: 0 }]);
  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.response.status, read.error?.code]).toEqual([404, "wiki.upgrade_not_found"]);
  const late = await postReject(api, dana, notebook.id);
  expect([late.response.status, late.error?.code]).toEqual([409, "wiki.upgrade_state"]);

  const reverted = await revertChangeset(api, pat, deleted);
  expect([reverted.response.status, reverted.error?.code]).toEqual([409, "page.virtual_file"]);
  const background = await postRevert(api, pat, notebook.id, deleted);
  expect([background.response.status, background.error?.code]).toEqual([409, "page.virtual_file"]);
  expect(
    await db.query(
      "SELECT name, deleted_at IS NOT NULL AS trashed FROM nodes WHERE notebook_id = $1 AND name = 'index'",
      [notebook.id]
    )
  ).toEqual([{ name: "index", trashed: true }]);
});

test("UP7 (MCP): the agent submits through upgrade_submit and a second admin approves; then the agent's list of the root starts with the LLM Wiki's virtual index.md and log.md, and its delete of AGENTS.md is page.protected", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { dana, notebook, agent } = await submitted(api, db, testInfo, nervewiki.baseURL, true);
  await approveUpgrade(api, dana, notebook.id);
  const listing = resultOf<{ items: { path: string; kind: string; virtual?: boolean }[] }>(
    await agent.call("list", {}),
    "the agent's list of the root"
  );
  expect(listing.items.slice(0, 2)).toEqual([
    { path: "index.md", kind: "page", has_children: false, virtual: true },
    { path: "log.md", kind: "page", has_children: false, virtual: true },
  ]);
  expect(errorOf(await agent.call("delete", { path: "AGENTS.md" }), "the agent's delete of AGENTS").code).toBe(
    "page.protected"
  );
});
