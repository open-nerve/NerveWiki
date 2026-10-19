import { bearer, emailFor } from "../../fixtures/auth";
import { getChangeset, listChanges, listChangesets, type LogQuery } from "../../fixtures/history";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf, type McpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getVersions, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// MC8, an agent reads the history over MCP (M9 design 9; M9/P2 design
// 4.6): the log narrowed by client, actor ("me" and another's id) and path,
// a page at a time; a changeset with its changes' paths; another
// notebook's changeset not found here; a page's versions, and one version's
// lines. Each answer is what REST answers: the story compares the two, not
// the database (M9 design 9, the MC8 exception).

/** A write's answer, as the tool's structuredContent has it. */
interface Written {
  path: string;
  node_id: string;
  version: string;
}

/** A move's answer. */
interface Moved {
  changeset_id: string | null;
}

/** A row of log, a change of changeset: as the wire has them, compared with REST's whole. */
type Row = Record<string, unknown>;

/** log's answer, changeset's, versions' list. */
interface Log {
  changesets: Row[];
  next_cursor: string | null;
}
interface ChangesetPage {
  changeset: Row | null;
  changes: { node: { id: string; kind: string; path: string | null } & Row }[];
  next_cursor: string | null;
}
interface Versions {
  path: string;
  node_id: string;
  current_version: string;
  versions: Row[];
  next_cursor: string | null;
}

/** The rows of agent's log of args, read a page at a time to its end. */
async function mcpLog(agent: McpClient, args: Record<string, unknown>): Promise<Row[]> {
  const rows: Row[] = [];
  for (let cursor: string | null = null, pages = 0; ; pages++) {
    expect(pages, `log ${JSON.stringify(args)} ends`).toBeLessThan(50);
    const sent: Record<string, unknown> = cursor === null ? args : { ...args, cursor };
    // oxlint-disable-next-line no-await-in-loop -- each page goes on from the one before
    const page: Log = resultOf<Log>(await agent.call("log", sent), `log ${JSON.stringify(sent)}`);
    rows.push(...page.changesets);
    if (page.next_cursor === null) {
      return rows;
    }
    cursor = page.next_cursor;
  }
}

test("MC8 (MCP): an agent reads the notebook's log narrowed by client, actor and path, a page at a time, a changeset with its nodes' paths, and a page's versions and one version's lines, each as REST answers them; another notebook's changeset is not found here", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const memberPat = await joinAs(api, pat, workspace.slug, emailFor(testInfo, "member"), "member");
  const member = (await api.GET("/api/v0/me", { headers: bearer(memberPat) })).data?.id;
  if (member === undefined) {
    throw new Error("me answered no account for the member");
  }
  const handbook = await createNotebook(api, pat, workspace.slug, "Handbook", "editor");
  const other = await createNotebook(api, pat, workspace.slug, "Other");
  const guides = await createPage(api, pat, handbook.id, "Guides");
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: handbook.id });
  const plan = resultOf<Written>(await agent.call("write", { path: "Plan.md", content: "# Plan\n" }), "write Plan");
  resultOf<Written>(
    await agent.call("write", { path: "Archive/Index.md", content: "# Index\n" }),
    "write Archive/Index"
  );
  resultOf<Written>(
    await agent.call("write", { path: "Plan.md", content: "# Plan\n\nship\n", base_version: plan.version }),
    "write Plan again"
  );
  resultOf(await agent.call("edit", { path: "Plan.md", old_string: "ship", new_string: "ship it" }), "edit Plan");
  const moved = resultOf<Moved>(await agent.call("move", { from: "Plan.md", to: "Archive/Plan.md" }), "move Plan");
  expect((await renameNode(api, memberPat, guides.id, "Guide")).response.status, "the member renames Guides").toBe(200);
  await createPage(api, pat, other.id, "Elsewhere", null, "e\n");

  /** REST's log of notebookId, as query filters it, read a page at a time to its end. */
  const restLog = async (notebookId: string, query: LogQuery): Promise<Row[]> => {
    const rows: Row[] = [];
    for (let cursor: string | undefined; ;) {
      // oxlint-disable-next-line no-await-in-loop -- each page goes on from the one before
      const { data, response } = await listChangesets(api, pat, notebookId, { ...query, limit: 100, cursor });
      expect(response.status, `REST's log of ${JSON.stringify(query)}`).toBe(200);
      rows.push(...((data?.data ?? []) as Row[]));
      if (!data?.next_cursor) {
        return rows;
      }
      cursor = data.next_cursor;
    }
  };

  // 1. The agents' changesets: the agent's five, not the member's rename.
  const agents = await mcpLog(agent, { client: "mcp" });
  expect(agents).toEqual(await restLog(handbook.id, { client: "mcp" }));
  expect(agents.map((r) => r.client)).toEqual(Array(5).fill("mcp:e2e-agent"));

  // 2. "me" is the token's account; another's id is theirs: the rename alone.
  expect(await mcpLog(agent, { actor: "me" })).toEqual(await restLog(handbook.id, { actor: adminId }));
  const renames = await mcpLog(agent, { actor: member });
  expect(renames).toEqual(await restLog(handbook.id, { actor: member }));
  expect(
    renames.map((r) => r.kind),
    "the member's one changeset"
  ).toEqual(["edit"]);

  // 3. Plan's changesets, by its path now, one a page.
  const plans = await mcpLog(agent, { path: "Archive/Plan.md", limit: 1 });
  expect(plans).toEqual(await restLog(handbook.id, { pageId: plan.node_id }));
  expect(plans).toHaveLength(4);

  // 4. The move: REST's changeset and changes, each node with its path now.
  const movedId = String(moved.changeset_id);
  const page = resultOf<ChangesetPage>(await agent.call("changeset", { id: movedId }), "changeset of the move");
  const { data: detail } = await getChangeset(api, pat, movedId);
  const { data: changes } = await listChanges(api, pat, movedId);
  const { changes: _first, ...header } = detail ?? { changes: [] };
  expect(page.changeset).toEqual(header);
  expect(page.changeset?.moves, "the move moves a node").toBe(true);
  expect(page.changes.map((c) => [c.node.id, c.node.kind, c.node.path])).toEqual([
    [plan.node_id, "page", "Archive/Plan.md"],
  ]);
  const [change] = changes?.data ?? [];
  expect(page.changes).toEqual([{ ...change, node: { ...change?.node, path: "Archive/Plan.md" } }]);
  expect(page.next_cursor).toBeNull();

  // 5. Another notebook's changeset, which the admin reads there, is not found here.
  const elsewhere = await restLog(other.id, {});
  expect(elsewhere, "Other's log: the admin's page").toHaveLength(1);
  expect(errorOf(await agent.call("changeset", { id: String(elsewhere[0]?.id) }), "changeset of Other's").code).toBe(
    "page.changeset_not_found"
  );

  // 6. Plan's versions, its version now read's.
  const read = await agent.call("read", { path: "Archive/Plan.md" });
  const { version } = JSON.parse(read.text[0] ?? "{}") as { version: string };
  const versions = resultOf<Versions>(await agent.call("versions", { path: "Archive/Plan.md" }), "versions of Plan");
  const { data: rest } = await getVersions(api, pat, plan.node_id);
  expect([versions.path, versions.node_id, versions.current_version, versions.next_cursor]).toEqual([
    "Archive/Plan.md",
    plan.node_id,
    version,
    null,
  ]);
  expect(versions.versions).toEqual(rest?.data);
  expect(versions.versions).toHaveLength(3);

  // 7. The first version's lines: the first write's content.
  const first = await agent.call("versions", { path: "Archive/Plan.md", revision: 1 });
  expect(first.isError, first.text.join(" ")).toBe(false);
  expect(first.text).toHaveLength(2);
  const meta = JSON.parse(first.text[0] ?? "{}") as { revision: number; current_version: string };
  expect([meta.revision, meta.current_version, first.text[1]]).toEqual([1, version, "# Plan\n"]);
});
