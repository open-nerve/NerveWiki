import { expectBaselineKept } from "../../fixtures/assert/page";
import type { Database } from "../../fixtures/db";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { startUpgrade } from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP4, the guardrails (M11/P1 design 5, M11/P2 design 5): the initiator's
// agent deletes no node the upgrade started with, nor the raw its start
// wrote; it deletes a page it wrote since (P1). It changes the
// frontmatter of a page the upgrade started with, not its body; it moves
// and renames with the links rewritten; a batch that folds a move's
// rewrite into such a page is refused, the same ops the other way round
// done (P2/S1). REST's writes are frozen before the guardrails: MCP alone.

/** A deletion's answer over MCP, as much as the story reads. */
interface Deleted {
  node_id: string;
  pages: number;
}

/** A write's answer over MCP, as much as the story reads. */
interface Written {
  node_id: string;
  version: string;
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

/** The content a page holds now and its version token, as an agent's write names it. */
async function pageNow(db: Database, id: string): Promise<{ content: string; version: string }> {
  const rows = await db.query<{ content: string; revision: number }>(
    "SELECT content, revision FROM page_contents WHERE node_id = $1",
    [id]
  );
  expect(rows, `the content of ${id}`).toHaveLength(1);
  return { content: rows[0]!.content, version: `${id}:${rows[0]!.revision}` };
}

test("UP4 (MCP): the initiator's agent changes a baseline page's frontmatter, not its body; it moves and renames with the links rewritten by the system; it writes a page it created as it likes; an invalid frontmatter is fixed in place, not pushed below another", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n\nThe plan.\n");
  // From the root, as a move writes it again: a bare name leads to Plan wherever it goes.
  const notes = await createPage(api, pat, notebook.id, "Notes", null, "See [the plan](/Plan.md) and [[Old]].\n");
  const old = await createPage(api, pat, notebook.id, "Old", null, "---\nOld text.\n");
  const broken = await createPage(api, pat, notebook.id, "Broken", null, "---\na: [\n---\nBroken text.\n");
  const other = await createPage(api, pat, notebook.id, "Other", null, "---\na: [\n---\nOther text.\n");
  const upgrade = await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  const code = async (what: string, tool: string, args: Record<string, unknown>) =>
    errorOf(await agent.call(tool, args), what).code;
  const done = async (what: string, tool: string, args: Record<string, unknown>) =>
    resultOf<Written>(await agent.call(tool, args), what);

  const planned = "---\ntype: note\nsummary: The plan.\n---\n# Plan\n\nThe plan.\n";
  await done("Plan's frontmatter", "write", {
    path: "Plan.md",
    content: planned,
    base_version: (await pageNow(db, plan.id)).version,
  });
  expect(
    await code("an edit of Plan's body", "edit", {
      path: "Plan.md",
      old_string: "# Plan\n\nThe plan.",
      new_string: "# Plan\n\nA plan.",
    })
  ).toBe("page.upgrade_immutable");
  expect(
    await code("Plan moved, its links kept", "move", { from: "Plan.md", to: "Moved.md", update_links: false })
  ).toBe("page.upgrade_keep_links");
  await done("the folder Work", "write", { path: "Work.md", content: "# Work\n" });
  await done("Plan moved into Work", "move", { from: "Plan.md", to: "Work/Plan.md" });
  await done("Old renamed to Legacy", "move", { from: "Old.md", to: "Legacy.md" });
  const draft = await done("a page of its own", "write", { path: "Draft.md", content: "# Draft\n" });
  await done("its body written again", "write", {
    path: "Draft.md",
    content: "# Draft\n\nMore.\n",
    base_version: draft.version,
  });
  const legacy = "---\ntype: note\n---\n---\nOld text.\n";
  await done("a frontmatter written above Legacy's lone opening line", "write", {
    path: "Legacy.md",
    content: legacy,
    base_version: (await pageNow(db, old.id)).version,
  });
  const fixed = '---\nraw: "a: ["\n---\nBroken text.\n';
  await done("Broken's frontmatter fixed in place", "write", {
    path: "Broken.md",
    content: fixed,
    base_version: (await pageNow(db, broken.id)).version,
  });
  expect(
    await code("a frontmatter written above Other's", "write", {
      path: "Other.md",
      content: "---\ntype: note\n---\n---\na: [\n---\nOther text.\n",
      base_version: (await pageNow(db, other.id)).version,
    })
  ).toBe("page.upgrade_immutable");

  const rows = await db.query<{ name: string; content: string }>(
    `SELECT n.name, c.content FROM nodes n JOIN page_contents c ON c.node_id = n.id
      WHERE n.id = ANY($1::uuid[]) ORDER BY n.name`,
    [[plan.id, notes.id, old.id, broken.id, other.id]]
  );
  expect(rows).toEqual([
    { name: "Broken", content: fixed },
    { name: "Legacy", content: legacy },
    { name: "Notes", content: "See [the plan](/Work/Plan.md) and [[Legacy]].\n" },
    { name: "Other", content: "---\na: [\n---\nOther text.\n" },
    { name: "Plan", content: planned },
  ]);
  await expectBaselineKept(db, upgrade.id, 5);
});

test("UP4 (MCP): a batch that moves Plan, then edits the frontmatter of Notes, which links to it, is page.upgrade_batch_rewrite at that edit, nothing written; the same two the other way round are done, Notes linking to Plan where it went", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");
  await createPage(api, pat, notebook.id, "Work", null, "# Work\n");
  const notes = await createPage(
    api,
    pat,
    notebook.id,
    "Notes",
    null,
    "---\ntype: note\n---\nSee [the plan](/Plan.md).\n"
  );
  const upgrade = await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  const state = async () =>
    db.query<{ name: string; parent_id: string | null; content: string; revision: number }>(
      `SELECT n.name, n.parent_id::text AS parent_id, c.content, c.revision FROM nodes n JOIN page_contents c ON c.node_id = n.id
        WHERE n.notebook_id = $1 AND n.deleted_at IS NULL ORDER BY n.name`,
      [notebook.id]
    );
  const before = await state();
  const move = { op: "move", from: "Plan.md", to: "Work/Plan.md" };
  const edit = { op: "edit", path: "Notes.md", old_string: "type: note", new_string: "type: summary" };
  const refused = errorOf(await agent.call("batch", { message: "Plan in Work", ops: [move, edit] }), "move, then edit");
  expect([refused.code, refused.op]).toEqual(["page.upgrade_batch_rewrite", 1]);
  expect(await state()).toEqual(before);

  resultOf<unknown>(await agent.call("batch", { message: "Plan in Work", ops: [edit, move] }), "edit, then move");
  expect((await pageNow(db, notes.id)).content).toBe("---\ntype: summary\n---\nSee [the plan](/Work/Plan.md).\n");
  await expectBaselineKept(db, upgrade.id, 3);
});
