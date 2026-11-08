import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { startUpgrade } from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP4, the guardrails (M11/P1 design 5): the initiator's agent deletes no
// node the upgrade started with, nor the raw its start wrote; it deletes
// a page it wrote since. P1 has the deletion's step; P2/S1 the rest.

/** A deletion's answer over MCP, as much as the story reads. */
interface Deleted {
  node_id: string;
  pages: number;
}

/** A write's answer over MCP, as much as the story reads. */
interface Written {
  node_id: string;
}

test("UP4 (MCP): the initiator's agent's deletion of a page of the baseline is page.upgrade_protected, so is raw's, which the start wrote; a page it wrote since it deletes; the two refused are alive", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");
  const upgrade = await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  const refusals = await Promise.all(["Plan.md", "raw.md"].map((path) => agent.call("delete", { path })));
  expect(refusals.map((answer, i) => errorOf(answer, `delete ${["Plan", "raw"][i]}`).code)).toEqual([
    "page.upgrade_protected",
    "page.upgrade_protected",
  ]);
  const draft = resultOf<Written>(await agent.call("write", { path: "Draft.md", content: "# Draft\n" }), "write Draft");
  const gone = resultOf<Deleted>(await agent.call("delete", { path: "Draft.md" }), "delete Draft");
  expect([gone.node_id, gone.pages]).toEqual([draft.node_id, 1]);

  const rows = await db.query<{ id: string; alive: boolean }>(
    "SELECT id::text AS id, deleted_at IS NULL AS alive FROM nodes WHERE id = ANY($1::uuid[]) ORDER BY id::text",
    [[plan.id, upgrade.source_root_id, draft.node_id]]
  );
  expect(rows).toEqual(
    [
      { id: plan.id, alive: true },
      { id: upgrade.source_root_id, alive: true },
      { id: draft.node_id, alive: false },
    ].toSorted((a, b) => (a.id < b.id ? -1 : 1))
  );
});
