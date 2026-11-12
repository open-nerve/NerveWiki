import type { ApiClient, Page, TransferJob, Upgrade } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectUpgrade } from "../../fixtures/assert/wiki";
import { emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { holdStream } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, type McpClient, mcpClient, resultOf } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, holdNotebookRow, listJobs, runningJob, startImport } from "../../fixtures/transfer";
import {
  bannerText,
  changeButtons,
  reviewDialog,
  upgradeBanner,
  upgradePath,
  upgradeSection,
  upgradeTitle,
} from "../../fixtures/upgrade-pages";
import {
  getUpgrade,
  postReject,
  postSendBack,
  rejectUpgrade,
  sendBackUpgrade,
  startUpgrade,
  submitUpgrade,
} from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// UP6, an upgrade in review sent back, then rejected (M11/P3 design 5): a
// second admin sends the submitted upgrade back, its submission and report
// cleared, the initiator's agent free to write again; submitted again, a
// report of its own; rejected, its rollback queued behind another
// notebook's import, which holds the server's one import worker, the
// notebook frozen until the rollback is done; then the notebook as it was
// when the upgrade started. Through the API, then through MCP, then on the
// upgrade's page (S3), in Chromium alone.

/** The story's notebook: Plan without frontmatter, and dana, a second admin, with a token of hers. */
async function reviewed(api: ApiClient, db: Database, testInfo: Parameters<typeof newTeam>[1]) {
  const { pat, workspace } = await newTeam(api, testInfo);
  const danaEmail = emailFor(testInfo, "dana");
  const dana = await joinAs(api, pat, workspace.slug, danaEmail, "member");
  const other = await createNotebook(api, pat, workspace.slug, "Other notes");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, danaEmail), "admin");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");
  return { pat, dana, other, notebook, plan };
}

/** reviewed's, dana onboarded: her tokens, for her page, and the workspace's slug. */
async function reviewedOnPage(api: ApiClient, db: Database, testInfo: Parameters<typeof newTeam>[1]) {
  const { pat, workspace } = await newTeam(api, testInfo);
  const danaEmail = emailFor(testInfo, "dana");
  const dana = await joinOnboarded(api, pat, workspace.slug, danaEmail, "member");
  const other = await createNotebook(api, pat, workspace.slug, "Other notes");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, danaEmail), "admin");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");
  return { pat, dana, slug: workspace.slug, other, notebook, plan };
}

/** The agent writes Plan's frontmatter with summary above its body, from the version the server holds. */
async function frontmatter(api: ApiClient, pat: string, agent: McpClient, plan: Page, summary: string) {
  const { revision } = await readContent(api, pat, plan.id);
  return agent.call("write", {
    path: "Plan.md",
    content: `---\ntype: note\nsummary: ${summary}\n---\n# Plan\n`,
    base_version: `${plan.id}:${revision}`,
  });
}

/** The id of the submission the notebook notebookId's upgrade under way holds, "" for none. */
async function submissionOf(db: Database, notebookId: string): Promise<string> {
  const rows = await db.query<{ id: string | null }>(
    "SELECT submission_id AS id FROM notebook_upgrades WHERE notebook_id = $1 AND state IN ('upgrading', 'reviewing')",
    [notebookId]
  );
  return rows[0]?.id ?? "";
}

/** The names of the nodes of the notebook notebookId in the trash, in order. */
async function trashedNames(db: Database, notebookId: string): Promise<string[]> {
  const rows = await db.query<{ name: string }>(
    "SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL ORDER BY name",
    [notebookId]
  );
  return rows.map((row) => row.name);
}

/**
 * Runs during with the server's one import worker held by an import into the notebook other, whose tree write
 * waits on the test's hold of its row: a rollback queued during stays queued. The hold is released, and the import
 * ended, in a finally.
 */
async function withWorkerHeld(
  api: ApiClient,
  db: Database,
  pat: string,
  other: string,
  testInfo: Parameters<typeof newTeam>[1],
  during: () => Promise<void>
): Promise<void> {
  let release: (() => Promise<void>) | undefined;
  const jobs: TransferJob[] = [];
  let failed = false;
  let cleanupErrors: unknown[] = [];
  try {
    release = await holdNotebookRow(db, other);
    const occupying = await startImport(api, pat, other, zipOf([{ name: "Other.md", data: "Other notes\n" }]));
    jobs.push(occupying);
    await runningJob(api, pat, occupying.id);
    await during();
  } catch (error) {
    failed = true;
    throw error;
  } finally {
    const released = await Promise.allSettled(release === undefined ? [] : [release()]);
    const finished = await Promise.allSettled(jobs.map((job) => endedJob(api, pat, job.id)));
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
}

/** The upgrade up rejected, as it ended once its rollback was done: rolled back, complete. */
function rolledBack(up: Upgrade): Upgrade {
  return { ...up, state: "rolled_back", rollback_complete: true, rollback_left: null };
}

test("UP6 (API): a second admin sends the submitted upgrade back, its submission and report cleared; the agent writes again and the initiator submits again, a report of its own; the second admin rejects it, its rollback queued behind another notebook's import; done, the notebook is as the upgrade started, the upgrade rolled back, rejected, its submission kept", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  test.slow();
  const { pat, dana, other, notebook, plan } = await reviewed(api, db, testInfo);
  await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  resultOf(await frontmatter(api, pat, agent, plan, "The plan."), "the agent's frontmatter of Plan");
  await submitUpgrade(api, pat, notebook.id);
  const first = await submissionOf(db, notebook.id);

  const back = await sendBackUpgrade(api, dana, notebook.id);
  expect([back.state, back.submitted_at, back.submitted_by, back.report, back.ending]).toEqual([
    "upgrading",
    null,
    null,
    null,
    null,
  ]);
  await expectUpgrade(db, back);
  const upgrading = await postReject(api, dana, notebook.id);
  expect([upgrading.response.status, upgrading.error?.code]).toEqual([409, "wiki.upgrade_state"]);
  resultOf(await frontmatter(api, pat, agent, plan, "The plan, again."), "the agent's write after the send-back");
  const again = await submitUpgrade(api, pat, notebook.id);
  expect([again.state, again.report?.passed]).toEqual(["reviewing", true]);
  const second = await submissionOf(db, notebook.id);
  expect(second).not.toBe("");
  expect(second).not.toBe(first);

  let rejected: Upgrade | undefined;
  await withWorkerHeld(api, db, pat, other.id, testInfo, async () => {
    rejected = await rejectUpgrade(api, dana, notebook.id);
    expect([rejected.state, rejected.ending, rejected.submitted_by?.user_id, rejected.report?.passed]).toEqual([
      "rolling_back",
      "rejected",
      again.submitted_by?.user_id,
      true,
    ]);
    const job = rejected.rollback_job_id ?? "";
    expect((await listJobs(api, pat, notebook.id)).find((j) => j.id === job)?.state).toBe("queued");
    const rolling = await postSendBack(api, dana, notebook.id);
    expect([rolling.response.status, rolling.error?.code]).toEqual([409, "wiki.upgrade_state"]);
  });
  if (!rejected) {
    throw new Error("the upgrade was not rejected");
  }
  const done = await endedJob(api, pat, rejected.rollback_job_id ?? "");
  expect([done.kind, done.state]).toEqual(["rollback", "succeeded"]);
  expect((await readContent(api, pat, plan.id)).content).toBe("# Plan\n");
  expect(await trashedNames(db, notebook.id)).toEqual(["AGENTS", "raw"]);
  await expectUpgrade(db, rolledBack(rejected));
});

test("UP6 (MCP): sent back, the agent's write passes and upgrade_submit keeps a new report; rejected, its rollback queued, the agent's write is notebook.upgrading; the rollback done, the agent writes again", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  test.slow();
  const { pat, dana, other, notebook, plan } = await reviewed(api, db, testInfo);
  await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  resultOf(await frontmatter(api, pat, agent, plan, "The plan."), "the agent's frontmatter of Plan");
  const submitted = resultOf<{ state: string }>(await agent.call("upgrade_submit", {}), "the agent's submission");
  expect(submitted.state).toBe("reviewing");
  const first = await submissionOf(db, notebook.id);

  await sendBackUpgrade(api, dana, notebook.id);
  resultOf(await frontmatter(api, pat, agent, plan, "The plan, again."), "the agent's write after the send-back");
  const again = resultOf<{ state: string }>(await agent.call("upgrade_submit", {}), "the agent's submission again");
  expect(again.state).toBe("reviewing");
  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.data?.state, read.data?.report?.passed]).toEqual(["reviewing", true]);
  expect(await submissionOf(db, notebook.id)).not.toBe(first);

  let job = "";
  await withWorkerHeld(api, db, pat, other.id, testInfo, async () => {
    job = (await rejectUpgrade(api, dana, notebook.id)).rollback_job_id ?? "";
    expect((await listJobs(api, pat, notebook.id)).find((j) => j.id === job)?.state).toBe("queued");
    const frozen = await frontmatter(api, pat, agent, plan, "Rejected.");
    expect(errorOf(frozen, "the agent's write while the rollback is queued").code).toBe("notebook.upgrading");
  });
  expect((await endedJob(api, pat, job)).state).toBe("succeeded");
  resultOf(
    await agent.call("write", { path: "After.md", content: "# After\n" }),
    "the agent's write once the rollback is done"
  );
  expect((await readContent(api, pat, plan.id)).content).toBe("# Plan\n");
});

test("UP6 (page): a second admin sends the submitted upgrade back on its page, the agent's section back; submitted again, the page, opened again with its stream held, offers Approve, Send back and Reject, not Abandon; rejected with the import worker held, it shows the rollback's job queued; the worker let go and the rollback done, it says no upgrade is under way, its one read of the ended upgrade 404", async ({
  api,
  db,
  nervewiki,
  signedInPage,
  pageWatch,
}, testInfo) => {
  test.slow();
  const { pat, dana, slug, other, notebook, plan } = await reviewedOnPage(api, db, testInfo);
  await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  resultOf(await frontmatter(api, pat, agent, plan, "The plan."), "the agent's frontmatter of Plan");
  await submitUpgrade(api, pat, notebook.id);
  const upgradeAt = upgradePath(slug, notebook.id);
  const upgradeRead = `/api/v0/notebooks/${notebook.id}/upgrade`;
  // The rollback done, its job seen ended reads the upgrade, ended, again (M11/P3 design 5): its stream held, no
  // mode event reads it, and the import worker held, the rejection's read again is of the rollback queued.
  pageWatch.expectConsole({ errors: [failedToLoad(404)] });
  const page = await signedInPage(dana);

  // Sent back on the page, its stream connected: upgrading again, the agent's section back.
  await page.goto(upgradeAt);
  await expect(upgradeBanner(page, bannerText.reviewing)).toBeVisible();
  await page.getByRole("button", { name: "Send back for more work…", exact: true }).click();
  const backDialog = reviewDialog(page, "Send back", notebook.name);
  const sentBack = answerTo(page, "POST", `${upgradeRead}/send-back`);
  await backDialog.getByRole("button", { name: "Send it back", exact: true }).click();
  const backAnswer = await sentBack;
  expect(backAnswer.status()).toBe(200);
  const back = (await backAnswer.json()) as Upgrade;
  expect([back.state, back.submitted_at, back.submitted_by, back.report]).toEqual(["upgrading", null, null, null]);
  await expectUpgrade(db, back);
  await expect(backDialog).toBeHidden();
  await expect(upgradeTitle(page)).toBeFocused();
  await expect(upgradeSection(page, "Connect an agent")).toBeVisible();
  await expect(upgradeBanner(page, bannerText.upgrading)).toBeVisible();

  resultOf(await frontmatter(api, pat, agent, plan, "The plan, again."), "the agent's write after the send-back");
  const again = await submitUpgrade(api, pat, notebook.id);
  // The submission's mode event read on the page before it is opened again: no read of it cut short by the load.
  await expect(upgradeBanner(page, bannerText.reviewing)).toBeVisible();
  await expect(upgradeSection(page, "Check report")).toContainText("): every check passed.");

  // Opened again, its stream held: what the rejection and its rollback send reach it as no event.
  const letStreamIn = await holdStream(page);
  try {
    await page.goto(upgradeAt);
    await expect(upgradeSection(page, "Check report")).toContainText("): every check passed.");
    expect(await changeButtons(page)).toEqual(["Approve…", "Send back for more work…", "Reject…"]);
    await expect(page.getByRole("button", { name: "Abandon this upgrade…", exact: true })).toHaveCount(0);

    let rejected: Upgrade | undefined;
    await withWorkerHeld(api, db, pat, other.id, testInfo, async () => {
      await page.getByRole("button", { name: "Reject…", exact: true }).click();
      const dialog = reviewDialog(page, "Reject", notebook.name);
      const answer = answerTo(page, "POST", `${upgradeRead}/reject`);
      await dialog.getByRole("button", { name: "Reject the upgrade", exact: true }).click();
      const response = await answer;
      expect(response.status()).toBe(200);
      rejected = (await response.json()) as Upgrade;
      expect([rejected.state, rejected.ending, rejected.submitted_by?.user_id, rejected.report?.passed]).toEqual([
        "rolling_back",
        "rejected",
        again.submitted_by?.user_id,
        true,
      ]);
      await expectUpgrade(db, rejected);
      const job = upgradeSection(page, "Rollback").getByRole("listitem", { name: /^Undo the upgrade, / });
      await expect(job.getByRole("progressbar", { name: "Queued", exact: true })).toBeVisible();
      await expect(dialog).toBeHidden();
      await expect(upgradeTitle(page)).toBeFocused();
      await expect(upgradeBanner(page, bannerText.rolling_back)).toBeVisible();
      const queued = (await listJobs(api, pat, notebook.id)).find((j) => j.id === rejected?.rollback_job_id);
      expect(queued?.state).toBe("queued");
    });
    if (!rejected) {
      throw new Error("the upgrade was not rejected on the page");
    }
    expect((await endedJob(api, pat, rejected.rollback_job_id ?? "")).state).toBe("succeeded");
    await expect(page.getByText("This notebook has no upgrade under way.", { exact: true })).toBeVisible();
    await expect.poll(() => pageWatch.apiFailures).toEqual([`404 GET ${upgradeRead}`]);
    await expectUpgrade(db, rolledBack(rejected));
    expect((await readContent(api, pat, plan.id)).content).toBe("# Plan\n");
    expect(await trashedNames(db, notebook.id)).toEqual(["AGENTS", "raw"]);

    // The page lets its stream in only once it has left the upgrade's page.
    await page.goto(notebookPath(slug, notebook.id));
    await expect(notebookHeading(page, notebook.name)).toBeVisible();
  } finally {
    letStreamIn();
  }
  await expect(upgradeBanner(page, bannerText.rolling_back)).toHaveCount(0);
  expect(pageWatch.apiFailures).toEqual([`404 GET ${upgradeRead}`]);
});
