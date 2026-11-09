import type { TransferJob, TransferJobDetail } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook } from "../../fixtures/assert/notebook";
import { upgradesOf } from "../../fixtures/assert/wiki";
import { displayNameOf, emailFor } from "../../fixtures/auth";
import { answerTo, failedToLoad } from "../../fixtures/browser";
import { joinAs } from "../../fixtures/invitations";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, endSession, listNodes, openSession } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, holdNotebookRow, listJobs, runningJob, startImport } from "../../fixtures/transfer";
import { modeSection, startDialog } from "../../fixtures/upgrade-pages";
import { postUpgrade, startUpgrade } from "../../fixtures/upgrades";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// UP2, a start refused (M11/P1 design 5): a page being edited, named in
// locks with its editor; a job writing the notebook's tree; an LLM Wiki;
// an upgrade under way. Each refusal writes nothing: no upgrade, the
// notebook as it was, no page written. The page version is S4's; MCP
// cannot start an upgrade.

test("UP2 (API): an editor's session is 409 wiki.upgrade_pages_locked naming the page and its editor; a queued import is 409 transfer.busy; an LLM Wiki is 409 notebook.already_llm_wiki; a second start is 409 notebook.upgrading; none writes anything, and once the editor and the import are done the start passes", async ({
  api,
  db,
}, testInfo) => {
  test.slow();
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editor = await joinAs(api, pat, workspace.slug, editorEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const other = await createNotebook(api, pat, workspace.slug, "Other notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, editorEmail), "editor");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");
  const nodes = (await listNodes(api, pat, notebook.id)).map((n) => n.id);
  const nothingWritten = async () => {
    expect(await upgradesOf(db, notebook.id)).toEqual([]);
    await expectNotebook(db, notebook, adminId);
    expect((await listNodes(api, pat, notebook.id)).map((n) => n.id)).toEqual(nodes);
  };

  const session = await openSession(api, editor, plan.id);
  const locked = await postUpgrade(api, pat, notebook.id);
  expect([locked.response.status, locked.error?.code, locked.error?.locks, locked.error?.detail]).toEqual([
    409,
    "wiki.upgrade_pages_locked",
    [{ page_id: plan.id, user_id: await accountIdOf(db, editorEmail), display_name: displayNameOf(editorEmail) }],
    "A page of the notebook is being edited: the upgrade can start once its editor leaves or an admin unlocks it.",
  ]);
  await nothingWritten();
  expect((await endSession(api, editor, session.id)).response.status).toBe(204);

  // The server has one import worker: the other notebook's import holds
  // it in a write of its tree, leaving this notebook's queued.
  let release: (() => Promise<void>) | undefined;
  const jobs: TransferJob[] = [];
  let finished: PromiseSettledResult<TransferJobDetail>[] = [];
  let cleanupErrors: unknown[] = [];
  let failed = false;
  try {
    release = await holdNotebookRow(db, other.id);
    const occupying = await startImport(api, pat, other.id, zipOf([{ name: "Other.md", data: "Other notes\n" }]));
    jobs.push(occupying);
    await runningJob(api, pat, occupying.id);
    const queued = await startImport(api, pat, notebook.id, zipOf([{ name: "Notes.md", data: "Field notes\n" }]));
    jobs.push(queued);
    expect((await listJobs(api, pat, notebook.id)).find((job) => job.id === queued.id)?.state).toBe("queued");
    const busy = await postUpgrade(api, pat, notebook.id);
    expect([busy.response.status, busy.error?.code]).toEqual([409, "transfer.busy"]);
    await nothingWritten();
  } catch (error) {
    failed = true;
    throw error;
  } finally {
    // Release before waiting, and drain every import even when an
    // assertion failed; a cleanup's failure does not hide that one.
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
    throw new AggregateError(cleanupErrors, "release the held notebook and finish every import");
  }
  expect(finished.map((result) => (result.status === "fulfilled" ? result.value.state : "rejected"))).toEqual([
    "succeeded",
    "succeeded",
  ]);

  const wiki = await createNotebook(api, pat, workspace.slug, "Wiki", undefined, { mode: "llm_wiki" });
  const already = await postUpgrade(api, pat, wiki.id);
  expect([already.response.status, already.error?.code]).toEqual([409, "notebook.already_llm_wiki"]);
  expect(await upgradesOf(db, wiki.id)).toEqual([]);

  const upgrade = await startUpgrade(api, pat, notebook.id);
  const again = await postUpgrade(api, pat, notebook.id);
  expect([again.response.status, again.error?.code]).toEqual([409, "notebook.upgrading"]);
  expect(await upgradesOf(db, notebook.id)).toEqual(["upgrading"]);
  expect(upgrade.seeded_page_ids).toHaveLength(2);
});

test("UP2 (page): the start's dialog says raw at the root becomes the source area, with the nodes under it; refused for an editor's session, it lists the page and its editor; that session ended, refused for a queued import, it says a job is writing the notebook; nothing is written", async ({
  api,
  db,
  signedInPage,
  pageWatch,
}, testInfo) => {
  test.slow();
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const editorEmail = emailFor(testInfo, "editor");
  const editor = await joinAs(api, pat, workspace.slug, editorEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const other = await createNotebook(api, pat, workspace.slug, "Other notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, editorEmail), "editor");
  const plan = await createPage(api, pat, notebook.id, "Plan", null, "# Plan\n");
  const raw = await createPage(api, pat, notebook.id, "raw");
  const paper = await createPage(api, pat, notebook.id, "Paper", raw.id, "# Paper\n");
  await createPage(api, pat, notebook.id, "Figures", paper.id, "# Figures\n");
  const session = await openSession(api, editor, plan.id);
  // The start's two refusals are the story's: the browser logs each answer of 409.
  pageWatch.expectConsole({ errors: [failedToLoad(409), failedToLoad(409)] });
  const page = await signedInPage(tokens);

  await page.goto(notebookPath(workspace.slug, notebook.id, "general"));
  await modeSection(page).getByRole("button", { name: "Upgrade to an LLM Wiki…", exact: true }).click();
  const dialog = startDialog(page, notebook.name);
  await expect(dialog).toContainText(
    "The page raw at the root becomes the source area (2 nodes under it now): once approved, everything under it is a read-only source."
  );
  const confirm = dialog.getByRole("button", { name: "Start the upgrade", exact: true });
  const locked = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/upgrade`);
  await confirm.click();
  expect((await locked).status()).toBe(409);
  await expect(dialog).toContainText("These pages are being edited, so the upgrade cannot start.");
  await expect(dialog.getByRole("listitem")).toHaveText([`${displayNameOf(editorEmail)} is editing “Plan”.`]);
  expect((await endSession(api, editor, session.id)).response.status).toBe(204);

  // The server has one import worker: the other notebook's import holds
  // it in a write of its tree, leaving this notebook's queued, which the
  // page has not read.
  let release: (() => Promise<void>) | undefined;
  const jobs: TransferJob[] = [];
  let cleanupErrors: unknown[] = [];
  let failed = false;
  try {
    release = await holdNotebookRow(db, other.id);
    const occupying = await startImport(api, pat, other.id, zipOf([{ name: "Other.md", data: "Other notes\n" }]));
    jobs.push(occupying);
    await runningJob(api, pat, occupying.id);
    const queued = await startImport(api, pat, notebook.id, zipOf([{ name: "Notes.md", data: "Field notes\n" }]));
    jobs.push(queued);
    expect((await listJobs(api, pat, notebook.id)).find((job) => job.id === queued.id)?.state).toBe("queued");
    const busy = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/upgrade`);
    await confirm.click();
    expect((await busy).status()).toBe(409);
    await expect(dialog).toContainText(
      "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again."
    );
    await expect(dialog.getByRole("listitem")).toHaveCount(0);
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
    throw new AggregateError(cleanupErrors, "release the held notebook and finish every import");
  }
  expect(await upgradesOf(db, notebook.id)).toEqual([]);
  await expectNotebook(db, notebook, adminId);
  expect(pageWatch.apiFailures).toEqual([
    `409 POST /api/v0/notebooks/${notebook.id}/upgrade`,
    `409 POST /api/v0/notebooks/${notebook.id}/upgrade`,
  ]);
});
