import { expectChangesetOf, expectNewPage } from "../../fixtures/assert/page";
import { lineAt, mcpClient, resultOf, type McpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { search } from "../../fixtures/search";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// MC6, an agent greps and searches a notebook over MCP (M9 design 9; M9/P4
// design 4.1): grep's hits by a regular expression with the line numbers
// read counts and the text without its line end, narrowed by a glob,
// without case, and page after page by its cursor; search's results,
// REST's search's of the notebook; and a page the agent wrote, found by
// both at once.

/** A grep's answer, as the tool's structuredContent has it. */
interface GrepAnswer {
  hits: { path: string; line: number; text: string; column?: number; cut?: boolean }[];
  partial_lines: { path: string; line: number }[];
  next_cursor: string | null;
  building: boolean;
  incomplete: boolean;
}

/** A search's answer. */
interface SearchAnswer {
  results: { path: string; kind: string; excerpt: string | null; changed_at: string }[];
  count: { value: number; kind: string } | null;
  next_cursor: string | null;
  building: boolean;
}

/** A write's answer, as much as the story reads. */
interface Written {
  node_id: string;
  changeset_id: string | null;
  at: string | null;
}

/** The client of the agent's changesets: the scripted client's name (fixtures/mcp.ts). */
const client = "mcp:e2e-agent";

/** grep's answer of args through agent, which must be no error. */
async function grep(agent: McpClient, args: Record<string, unknown>): Promise<GrepAnswer> {
  return resultOf<GrepAnswer>(await agent.call("grep", args), `grep ${JSON.stringify(args)}`);
}

/** search's answer of args through agent, which must be no error. */
async function searchOf(agent: McpClient, args: Record<string, unknown>): Promise<SearchAnswer> {
  return resultOf<SearchAnswer>(await agent.call("search", args), `search ${JSON.stringify(args)}`);
}

/** The hits of answer as "path:line text". */
function hitsOf(answer: GrepAnswer): string[] {
  return answer.hits.map((h) => `${h.path}:${h.line} ${h.text}`);
}

test("MC6 (MCP): an agent greps by a regular expression, the lines numbered as read counts them, narrowed by a glob, without case, page after page; searches as REST's search of the notebook; finds a page it wrote at once with both", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const notes = await createPage(api, pat, notebook.id, "Notes");
  const alpha = await createPage(
    api,
    pat,
    notebook.id,
    "Alpha",
    notes.id,
    "---\r\ntype: note\r\n---\r\n# Alpha\r\nTODO: alpha one\r\nplain\r\n"
  );
  const beta = await createPage(api, pat, notebook.id, "Beta", notes.id, "todo: beta\n数据库设计说明\n");
  const other = await createPage(api, pat, notebook.id, "Other");
  await createPage(api, pat, notebook.id, "Gamma", other.id, "TODO: gamma\n");
  const stock = await createPage(api, pat, notebook.id, "库存", null, "warehouse counts\n");
  const dates = await createPage(api, pat, notebook.id, "Dates");
  await Promise.all(
    Array.from({ length: 12 }, (_, i) =>
      createPage(api, pat, notebook.id, `D${String(i + 1).padStart(2, "0")}`, dates.id, `2026-10-${10 + i}\n`)
    )
  );
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  // 1. The regular expression's lines, in path order, each the line read gives at its number without its line end.
  const todos = await grep(agent, { pattern: "TODO: \\w+" });
  expect(hitsOf(todos)).toEqual(["Notes/Alpha.md:5 TODO: alpha one", "Other/Gamma.md:1 TODO: gamma"]);
  expect([todos.partial_lines, todos.next_cursor, todos.building, todos.incomplete]).toEqual([[], null, false, false]);
  expect(await lineAt(agent, "Notes/Alpha.md", 5)).toBe("TODO: alpha one\r\n");
  expect(await lineAt(agent, "Other/Gamma.md", 1)).toBe("TODO: gamma\n");

  // 2. The glob narrows the pages.
  expect(hitsOf(await grep(agent, { pattern: "TODO: \\w+", glob: "Notes/**" }))).toEqual([
    "Notes/Alpha.md:5 TODO: alpha one",
  ]);

  // 3. Without case.
  expect(hitsOf(await grep(agent, { pattern: "todo: \\w+", ignore_case: true }))).toEqual([
    "Notes/Alpha.md:5 TODO: alpha one",
    "Notes/Beta.md:1 todo: beta",
    "Other/Gamma.md:1 TODO: gamma",
  ]);

  // 4. A pattern of no literal, five at a time: the cursor goes on to the twelfth date, none twice.
  const dated: string[] = [];
  const args: Record<string, unknown> = { pattern: "\\d{4}-\\d{2}-\\d{2}", limit: 5 };
  for (let calls = 0; ; calls++) {
    expect(calls, "grep's pages of dates").toBeLessThan(10);
    // oxlint-disable-next-line no-await-in-loop -- each page goes on from the cursor of the one before
    const page = await grep(agent, args);
    expect(page.building).toBe(false);
    dated.push(...page.hits.map((h) => `${h.path}:${h.line}`));
    if (calls === 0) {
      expect(dated).toEqual(["Dates/D01.md:1", "Dates/D02.md:1", "Dates/D03.md:1", "Dates/D04.md:1", "Dates/D05.md:1"]);
      expect(page.next_cursor).not.toBeNull();
    }
    if (page.next_cursor === null) {
      break;
    }
    args.cursor = page.next_cursor;
  }
  expect(dated).toEqual(Array.from({ length: 12 }, (_, i) => `Dates/D${String(i + 1).padStart(2, "0")}.md:1`));

  // 5. search is REST's search of the notebook: the same nodes in the same order, the same count; one Chinese
  // character finds titles alone.
  const byPath = new Map([
    ["Notes.md", notes.id],
    ["Notes/Alpha.md", alpha.id],
    ["Notes/Beta.md", beta.id],
    ["库存.md", stock.id],
  ]);
  const found = await searchOf(agent, { q: "alpha" });
  const { data: rest } = await search(api, pat, workspace.slug, "alpha", { notebookId: notebook.id });
  expect(found.results.map((r) => byPath.get(r.path) ?? r.path)).toEqual(rest?.data.map((h) => h.node.id));
  expect(found.count).toEqual(rest?.count);
  expect(found.count?.kind).toBe("exact");
  const alphaResult = found.results.find((r) => r.path === "Notes/Alpha.md");
  expect(alphaResult?.excerpt ?? "").toContain("alpha");
  expect((await searchOf(agent, { q: "数据库" })).results.map((r) => r.path)).toEqual(["Notes/Beta.md"]);
  expect((await searchOf(agent, { q: "库" })).results.map((r) => r.path)).toEqual(["库存.md"]);

  // 6. What the agent writes, grep and search find at once, queued exact; the write is the agent's, in the database.
  const fresh = resultOf<Written>(
    await agent.call("write", { path: "Notes/Fresh.md", content: "zebra crossing\n" }),
    "write Fresh"
  );
  expect(hitsOf(await grep(agent, { pattern: "zebra" }))).toEqual(["Notes/Fresh.md:1 zebra crossing"]);
  expect((await searchOf(agent, { q: "zebra" })).results.map((r) => r.path)).toEqual(["Notes/Fresh.md"]);
  await expectNewPage(
    db,
    { id: fresh.node_id, notebook_id: notebook.id, parent_id: notes.id, name: "Fresh", created_at: String(fresh.at) },
    adminId,
    client,
    "zebra crossing\n"
  );
  await expectChangesetOf(db, String(fresh.changeset_id), {
    client,
    byId: adminId,
    items: [fresh.node_id],
    versions: [fresh.node_id],
  });
});
