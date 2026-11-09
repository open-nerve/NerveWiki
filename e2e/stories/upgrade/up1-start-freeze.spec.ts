import type { TreeNode } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook } from "../../fixtures/assert/notebook";
import { expectBaseline, expectChangesetOf } from "../../fixtures/assert/page";
import { expectUpgrade } from "../../fixtures/assert/wiki";
import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { emailFor } from "../../fixtures/auth";
import { listChangesets } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { createNotebook, getNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes, postPage, putContent, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { getUpgrade, startUpgrade } from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP1, an upgrade's start and its freeze (M11/P1 design 5): an admin
// starts the upgrade of a standard notebook; the start writes the raw and
// AGENTS it lacks as the upgrade's own, and keeps what was there as the
// baseline; then no REST write passes, the initiator's either, and over
// MCP only the initiator's agent writes. The page version is S4's.

/** A write's answer over MCP, as much as the story reads. */
interface Written {
  node_id: string;
  changeset_id: string | null;
}

/** The node of nodes at the root titled name. */
function rootNode(nodes: TreeNode[], name: string): TreeNode {
  const node = nodes.find((n) => n.parent_id === null && n.name === name);
  if (!node) {
    throw new Error(`no ${name} at the root of ${JSON.stringify(nodes.map((n) => n.name))}`);
  }
  return node;
}

test("UP1 (API): an admin starts the upgrade of a standard notebook of two pages and an attachment; raw and AGENTS are written in a changeset of the upgrade, raw protected, the three nodes its baseline; the notebook and the upgrade read it; an editor's REST writes and the initiator's are 409 notebook.upgrading and write nothing", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editor = await joinAs(api, pat, workspace.slug, editorEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, editorEmail), "editor");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "See [[Roadmap]].\n");
  const notes = await createPage(api, pat, notebook.id, "Notes", plan.id, "# Notes\n");
  const diagram = await uploadAsset(api, pat, notebook.id, { name: "diagram.png", bytes: pngBytes }, plan.id);

  const upgrade = await startUpgrade(api, pat, notebook.id);
  const nodes = await listNodes(api, pat, notebook.id);
  const raw = rootNode(nodes, "raw");
  const agents = rootNode(nodes, "AGENTS");
  expect([
    upgrade.notebook_id,
    upgrade.state,
    upgrade.source_root_id,
    upgrade.seeded_page_ids,
    upgrade.baseline,
    upgrade.started_by.user_id,
    upgrade.ending,
    upgrade.rollback_job_id,
  ]).toEqual([
    notebook.id,
    "upgrading",
    raw.id,
    [raw.id, agents.id],
    { nodes: 3, pages: 2, wanted: 1 },
    adminId,
    null,
    null,
  ]);
  await expectUpgrade(db, upgrade);
  await expectBaseline(db, upgrade.id, { baseline: [plan.id, notes.id, diagram.id], protected: [raw.id] });
  const log = await listChangesets(api, pat, notebook.id);
  expect(log.response.status).toBe(200);
  await expectChangesetOf(db, log.data?.data[0]?.id ?? "", {
    client: "api",
    byId: adminId,
    items: [raw.id, agents.id],
    versions: [raw.id, agents.id],
    message: "LLM Wiki template",
    kind: "edit",
    upgradeId: upgrade.id,
  });
  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.response.status, read.data]).toEqual([200, upgrade]);
  const frozen = await getNotebook(api, pat, notebook.id);
  if (!frozen.data) {
    throw new Error(`the notebook read ${frozen.response.status} after the start`);
  }
  expect([
    frozen.data.mode,
    frozen.data.upgrade?.id,
    frozen.data.upgrade?.state,
    frozen.data.upgrade?.started_by,
  ]).toEqual(["standard", upgrade.id, "upgrading", adminId]);
  await expectNotebook(db, frozen.data, adminId);

  const writes = [
    await postPage(api, editor, notebook.id, { parent_id: null, title: "Roadmap" }),
    await putContent(api, editor, plan.id, { content: "Edited.\n", base_revision: 1 }),
    await putContent(api, pat, plan.id, { content: "Edited by its admin.\n", base_revision: 1 }),
    await postPage(api, pat, notebook.id, { parent_id: null, title: "Roadmap" }),
  ];
  expect(writes.map((w) => [w.response.status, w.error?.code])).toEqual(writes.map(() => [409, "notebook.upgrading"]));
  expect((await readContent(api, pat, plan.id)).revision).toBe(1);
  expect((await listNodes(api, pat, notebook.id)).map((n) => n.id).toSorted()).toEqual(
    nodes.map((n) => n.id).toSorted()
  );
});

test("UP1 (MCP): while an admin's upgrade is under way, the initiator's agent writes a page, in a changeset of the upgrade; another admin's agent's write is notebook.upgrading and writes nothing", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const otherEmail = emailFor(testInfo, "other");
  const other = await joinAs(api, pat, workspace.slug, otherEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, otherEmail), "admin");
  await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");

  const upgrade = await startUpgrade(api, pat, notebook.id);
  const initiators = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  const written = resultOf<Written>(
    await initiators.call("write", { path: "Overview.md", content: "# Overview\n" }),
    "the initiator's agent writes Overview"
  );
  expect((await readContent(api, pat, written.node_id)).content).toBe("# Overview\n");
  await expectChangesetOf(db, String(written.changeset_id), {
    client: "mcp:e2e-agent",
    byId: adminId,
    items: [written.node_id],
    versions: [written.node_id],
    upgradeId: upgrade.id,
  });

  const others = await mcpClient(nervewiki.baseURL, other, { notebook: notebook.id });
  const refused = errorOf(
    await others.call("write", { path: "Elsewhere.md", content: "# Elsewhere\n" }),
    "another admin's agent writes Elsewhere"
  );
  expect(refused.code).toBe("notebook.upgrading");
  expect((await listNodes(api, pat, notebook.id)).map((n) => n.name).toSorted()).toEqual([
    "AGENTS",
    "Overview",
    "Plan",
    "raw",
  ]);
});
