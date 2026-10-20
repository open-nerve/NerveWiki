import { displayNameOf, emailFor } from "../../fixtures/auth";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import { expectChangesetOf, expectNoAsideNames } from "../../fixtures/assert/page";
import { joinAs } from "../../fixtures/invitations";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, openSession, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// MC5, an agent's batch (M9 design 9; M9/P3 design 4.2): many writes as one
// change with a message, the links following its rename once, and a batch
// refused whole at the op whose page a teammate is editing.

/** An op's result in batch's answer, as the tool's structuredContent has it. */
interface OpResult {
  op: string;
  path: string;
  node_id: string;
  changed: boolean;
  version?: string;
  created?: { path: string; node_id: string }[];
  pages?: number;
}

/** batch's answer. */
interface Batched {
  changeset_id: string | null;
  at: string | null;
  changed: boolean;
  results: OpResult[];
  results_count: number;
}

/** log's answer, its rows as much as the story reads. */
interface Log {
  changesets: { id: string; message: string | null }[];
}

test("MC5 (MCP): an agent ingests two papers in one batch with a message, one changeset, Index's links following the rename; a batch writing a page a teammate is editing is refused whole, naming the op and the holder", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const beaEmail = emailFor(testInfo, "bea");
  const bea = await joinAs(api, pat, workspace.slug, beaEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Research", "editor");
  const index = await createPage(api, pat, notebook.id, "Index", null, "[[Draft]] and [[Old]]\n");
  const draft = await createPage(api, pat, notebook.id, "Draft");
  const old = await createPage(api, pat, notebook.id, "Old");
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  const ingest = resultOf<Batched>(
    await agent.call("batch", {
      message: "Ingest two papers",
      ops: [
        { op: "write", path: "Sources/Paper A.md", content: "# Paper A\n" },
        { op: "write", path: "Sources/Paper B.md", content: "# Paper B\n" },
        { op: "write", path: "Notes/Summary.md", content: "[[Paper A]] [[Paper B]]\n" },
        { op: "edit", path: "Index.md", old_string: "[[Old]]\n", new_string: "[[Old]]\n- [[Summary]]\n" },
        { op: "move", from: "Draft.md", to: "Plan.md" },
        { op: "delete", path: "Old.md" },
      ],
    }),
    "the ingest's batch"
  );
  const [paperA, paperB, summary, edited, plan, deleted] = ingest.results;
  expect([ingest.changed, ingest.results_count, ingest.results.length]).toEqual([true, 6, 6]);
  expect(paperA?.created?.map((c) => c.path)).toEqual(["Sources.md", "Sources/Paper A.md"]);
  expect(summary?.created?.map((c) => c.path)).toEqual(["Notes.md", "Notes/Summary.md"]);
  const indexNow = await readContent(api, pat, index.id);
  expect(edited?.version, "Index's version, after the deferred rewrite").toBe(`${index.id}:${indexNow.revision}`);
  expect([plan?.path, plan?.node_id, deleted?.pages, deleted?.node_id]).toEqual(["Plan.md", draft.id, 1, old.id]);

  const created = [...(paperA?.created ?? []), ...(paperB?.created ?? []), ...(summary?.created ?? [])].map(
    (c) => c.node_id
  );
  expect(created).toHaveLength(5);
  await expectChangesetOf(db, String(ingest.changeset_id), {
    client: "mcp:e2e-agent",
    byId: adminId,
    items: [...created, draft.id, old.id],
    versions: [...created, index.id],
    message: "Ingest two papers",
    kind: "edit",
  });
  expect(indexNow.content).toBe("[[Plan]] and [[Old]]\n- [[Summary]]\n");
  await expectIndexedLinks(db, index.id, [
    { kind: "wikilink", property: null, target: "Plan", resolved: draft.id },
    { kind: "wikilink", property: null, target: "Old", resolved: null },
    { kind: "wikilink", property: null, target: "Summary", resolved: summary?.node_id ?? null },
  ]);
  const log = resultOf<Log>(await agent.call("log", { limit: 1 }), "log");
  expect(log.changesets.map((c) => [c.id, c.message])).toEqual([[ingest.changeset_id, "Ingest two papers"]]);

  await openSession(api, bea, index.id);
  const changesets = async () =>
    (
      await db.query<{ n: number }>(`SELECT count(*)::int AS n FROM changesets WHERE notebook_id = $1`, [notebook.id])
    )[0]?.n;
  const before = await changesets();
  const refused = errorOf(
    await agent.call("batch", {
      message: "Add a third paper",
      ops: [
        { op: "write", path: "Sources/Paper C.md", content: "# Paper C\n" },
        { op: "edit", path: "Index.md", old_string: "- [[Summary]]\n", new_string: "- [[Summary]]\n- [[Paper C]]\n" },
      ],
    }),
    "the batch while Bea edits Index"
  );
  expect([refused.code, refused.op, (refused.lock as { display_name: string }).display_name]).toEqual([
    "page.locked",
    1,
    displayNameOf(beaEmail),
  ]);
  expect(refused.message).toContain("ops[1] (edit)");
  expect(await db.query(`SELECT id FROM nodes WHERE notebook_id = $1 AND name = 'Paper C'`, [notebook.id])).toEqual([]);
  expect(await changesets()).toBe(before);
  await expectNoAsideNames(db, notebook.id);
});
