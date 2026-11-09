import type { TransferJob, TransferJobDetail, TreeNode, Upgrade } from "@nervewiki/api-client";

import { expectBackgroundJob } from "../../fixtures/assert/transfer";
import { expectUpgrade } from "../../fixtures/assert/wiki";
import type { Database } from "../../fixtures/db";
import { mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, listNodes, postPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { cancelJob, endedJob, holdNotebookRow, listJobs, runningJob, startImport } from "../../fixtures/transfer";
import { abandonUpgrade, endUpgradeRollback, getUpgrade, startUpgrade } from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// UP3, an upgrade abandoned and rolled back (M11/P1 design 5): its
// rollback, a background job, puts the notebook back as the upgrade
// started; one stopped is continued, or ended as it is. The page version
// is S4's.

/** A notebook's tree and its pages' contents, as the API reads them: what a rollback puts back. */
async function treeAndContents(
  api: Parameters<typeof listNodes>[0],
  credential: string,
  notebookId: string
): Promise<string[]> {
  const nodes = await listNodes(api, credential, notebookId);
  return Promise.all(
    nodes.map(async (node: TreeNode) =>
      [
        node.id,
        node.parent_id ?? "root",
        node.name,
        node.kind === "page" ? (await readContent(api, credential, node.id)).content : "",
      ].join(" ")
    )
  );
}

/** The names of the nodes of the notebook notebookId in the trash, in order. */
async function trashedNames(db: Database, notebookId: string): Promise<string[]> {
  const rows = await db.query<{ name: string }>(
    "SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL ORDER BY name",
    [notebookId]
  );
  return rows.map((row) => row.name);
}

/** The upgrade up as it ended, rolled back, as the API would have answered it: complete, or with left. */
function rolledBack(up: Upgrade, job: string, left: Upgrade["rollback_left"]): Upgrade {
  return { ...up, state: "rolled_back", rollback_job_id: job, rollback_complete: left === null, rollback_left: left };
}

test("UP3 (API): an admin's agent moves, renames, rewrites the frontmatter of and writes pages; the admin abandons the upgrade; its rollback deletes the new pages, then puts the baseline's places, names and contents back, in one changeset of the upgrade; the upgrade ends rolled back, complete, and the notebook is writable again", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "---\nstatus: draft\n---\n# Plan\n");
  await createPage(api, pat, notebook.id, "Notes", plan.id, "# Notes\n");
  await createPage(api, pat, notebook.id, "Guide", null, "See [[Plan]].\n");
  const before = await treeAndContents(api, pat, notebook.id);

  const upgrade = await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  resultOf(await agent.call("write", { path: "Hub/Index.md", content: "# Index\n" }), "the agent writes Hub/Index");
  resultOf(await agent.call("move", { from: "Guide.md", to: "Hub/Guide.md" }), "the agent moves Guide under Hub");
  resultOf(await agent.call("move", { from: "Plan/Notes.md", to: "Plan/Field notes.md" }), "the agent renames Notes");
  resultOf(
    await agent.call("edit", { path: "Plan.md", old_string: "status: draft", new_string: "status: wiki" }),
    "the agent rewrites Plan's frontmatter"
  );
  resultOf(await agent.call("write", { path: "Fresh.md", content: "# Fresh\n" }), "the agent writes Fresh");

  const abandoned = await abandonUpgrade(api, pat, notebook.id);
  expect([
    abandoned.id,
    abandoned.state,
    abandoned.ending,
    abandoned.rollback_complete,
    abandoned.rollback_left,
  ]).toEqual([upgrade.id, "rolling_back", "abandoned", null, null]);
  const job = abandoned.rollback_job_id ?? "";
  const ended = await endedJob(api, pat, job);
  expect([ended.kind, ended.name, ended.state]).toEqual(["rollback", "upgrade", "succeeded"]);
  expect(await treeAndContents(api, pat, notebook.id)).toEqual(before);
  expect(await trashedNames(db, notebook.id)).toEqual(["AGENTS", "Fresh", "Hub", "Index", "raw"]);
  const changeset = await expectBackgroundJob(db, job, {
    kind: "rollback",
    upgrade: upgrade.id,
    creatorId: adminId,
    client: "api",
    root: null,
    target: null,
    // Raw, AGENTS, Index and Fresh deleted first; then Hub deleted, Guide and Notes put back, Plan written back.
    nodes: 8,
    counts: { deleted: 5, put_back: 2, written: 1 },
    placed: null,
  });
  const rollbacks = await db.query<{ id: string }>(
    "SELECT id FROM changesets WHERE notebook_id = $1 AND kind = 'rollback' AND upgrade_id = $2",
    [notebook.id, upgrade.id]
  );
  expect(rollbacks).toEqual([{ id: changeset }]);
  await expectUpgrade(db, rolledBack(abandoned, job, null));
  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.response.status, read.error?.code]).toEqual([404, "wiki.upgrade_not_found"]);
  const after = await postPage(api, pat, notebook.id, { parent_id: null, title: "After" });
  expect(after.response.status).toBe(201);
});

test("UP3 (API): with the server's import worker held by another notebook's import, two upgrades' rollbacks wait queued; an admin cancels both; one is continued and done, the other ended as it is, rolled back, not complete, with what was left; both notebooks are writable again", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  test.slow();
  const { pat, workspace } = await newTeam(api, testInfo);
  const other = await createNotebook(api, pat, workspace.slug, "Other notes");
  const continued = await createNotebook(api, pat, workspace.slug, "Continued");
  const ended = await createNotebook(api, pat, workspace.slug, "Ended");
  await Promise.all([continued, ended].map((notebook) => createPage(api, pat, notebook.id, "Plan", null, "# Plan\n")));
  const before = await treeAndContents(api, pat, continued.id);
  await Promise.all([continued, ended].map((notebook) => startUpgrade(api, pat, notebook.id)));
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: ended.id });
  resultOf(await agent.call("move", { from: "Plan.md", to: "Plan 2.md" }), "the agent's rename of Plan");
  resultOf(await agent.call("write", { path: "Fresh.md", content: "# Fresh\n" }), "the agent's write of Fresh");

  // The server has one import worker, which the rollbacks share: the other
  // notebook's import holds it in a write of its tree, leaving them queued.
  let release: (() => Promise<void>) | undefined;
  const jobs: TransferJob[] = [];
  const stopped = new Map<string, Upgrade>();
  // Abandoned, the notebook's rollback waits queued, and its admin cancels it.
  const queuedAndCancelled = async (notebookId: string) => {
    const abandoned = await abandonUpgrade(api, pat, notebookId);
    const job = abandoned.rollback_job_id ?? "";
    expect((await listJobs(api, pat, notebookId)).find((j) => j.id === job)?.state).toBe("queued");
    const cancelled = await cancelJob(api, pat, job);
    expect([cancelled.response.status, cancelled.data?.state]).toEqual([200, "cancelled"]);
    stopped.set(notebookId, abandoned);
  };
  let finished: PromiseSettledResult<TransferJobDetail>[] = [];
  let cleanupErrors: unknown[] = [];
  let failed = false;
  try {
    release = await holdNotebookRow(db, other.id);
    const occupying = await startImport(api, pat, other.id, zipOf([{ name: "Other.md", data: "Other notes\n" }]));
    jobs.push(occupying);
    await runningJob(api, pat, occupying.id);
    await queuedAndCancelled(continued.id);
    await queuedAndCancelled(ended.id);
  } catch (error) {
    failed = true;
    throw error;
  } finally {
    const released = await Promise.allSettled(release === undefined ? [] : [release()]);
    finished = await Promise.allSettled(jobs.map((job) => endedJob(api, pat, job.id)));
    cleanupErrors = [...released, ...finished].flatMap((result) =>
      result.status === "rejected" ? [result.reason as unknown] : []
    );
    if (failed && cleanupErrors.length > 0) {
      testInfo.annotations.push({ type: "cleanup", description: cleanupErrors.map(String).join("\n") });
    }
  }
  if (cleanupErrors.length > 0) {
    throw new AggregateError(cleanupErrors, "release the held notebook and finish the import");
  }

  // Stopped, each still rolls back, frozen, and tells what is left.
  const reads = await Promise.all([continued, ended].map((notebook) => getUpgrade(api, pat, notebook.id)));
  expect(reads.map((read) => [read.response.status, read.data?.state, read.data?.rollback_left])).toEqual([
    [200, "rolling_back", { nodes: 0, pages: 0, created: 2 }],
    [200, "rolling_back", { nodes: 1, pages: 0, created: 3 }],
  ]);
  const frozen = await Promise.all(
    [continued, ended].map((notebook) => postPage(api, pat, notebook.id, { parent_id: null, title: "Frozen" }))
  );
  expect(frozen.map((f) => [f.response.status, f.error?.code])).toEqual([
    [409, "notebook.upgrading"],
    [409, "notebook.upgrading"],
  ]);

  const again = await abandonUpgrade(api, pat, continued.id);
  const second = again.rollback_job_id ?? "";
  expect(second).not.toBe(stopped.get(continued.id)?.rollback_job_id);
  expect((await endedJob(api, pat, second)).state).toBe("succeeded");
  expect(await treeAndContents(api, pat, continued.id)).toEqual(before);
  await expectUpgrade(db, rolledBack(again, second, null));

  const end = await endUpgradeRollback(api, pat, ended.id);
  const left = { nodes: 1, pages: 0, created: 3 };
  expect([end.state, end.rollback_complete, end.rollback_left]).toEqual(["rolled_back", false, left]);
  const first = stopped.get(ended.id);
  if (!first) {
    throw new Error("the ended notebook's rollback was not stopped");
  }
  await expectUpgrade(db, rolledBack(first, first.rollback_job_id ?? "", left));
  expect((await listNodes(api, pat, ended.id)).map((n) => n.name).toSorted()).toEqual([
    "AGENTS",
    "Fresh",
    "Plan 2",
    "raw",
  ]);
  const after = await Promise.all(
    [continued, ended].map((notebook) => postPage(api, pat, notebook.id, { parent_id: null, title: "After" }))
  );
  expect(after.map((a) => a.response.status)).toEqual([201, 201]);
});
