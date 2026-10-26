import type { TransferJob, TransferJobDetail } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectNotebook } from "../../fixtures/assert/notebook";
import { expectSubtreeDeleted } from "../../fixtures/assert/page";
import { emailFor } from "../../fixtures/auth";
import { answerTo, countAnswers } from "../../fixtures/browser";
import { followStreams, holdStream, settleEvents } from "../../fixtures/events";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { downgradeWith, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook, downgradeNotebook, getNotebook } from "../../fixtures/notebooks";
import { deleteNode, listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { endedJob, holdNotebookRow, listJobs, runningJob, startImport } from "../../fixtures/transfer";
import { pageTree } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";
import { zipOf } from "../../fixtures/zip-write";

// LW12, a downgrade (M10/P1 design 5.2): even a queued import keeps its
// notebook's mode; once finished, a downgrade reaches readers through
// the mode event and makes AGENTS and raw ordinary pages.

test("LW12 (API): a queued import refuses a downgrade; afterward an admin makes the notebook standard, its readers receive mode, its protected pages can be deleted, and a writer still cannot downgrade", async ({
  api,
  db,
  nervewiki,
  openEvents,
}, testInfo) => {
  test.slow();
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes", "none", { mode: "llm_wiki" });
  const other = await createNotebook(api, pat, workspace.slug, "Other notes");
  const readerEmail = emailFor(testInfo, "reader");
  const writerEmail = emailFor(testInfo, "writer");
  const readerPat = await joinAs(api, pat, workspace.slug, readerEmail, "member");
  const writerPat = await joinAs(api, pat, workspace.slug, writerEmail, "member");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, readerEmail), "reader");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, writerEmail), "editor");
  const [raw, agents] = await listNodes(api, pat, notebook.id);
  if (raw?.name !== "raw" || agents?.name !== "AGENTS" || raw.id !== notebook.source_root_id) {
    throw new Error("the LLM Wiki must have raw, its source root, then AGENTS");
  }

  // The default server has one import worker. B holds it in a tree write,
  // leaving A's persisted job queued, without holding A's notebook row.
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
    const before = await getNotebook(api, pat, notebook.id);
    expect(before.response.status).toBe(200);
    if (!before.data) {
      throw new Error("the notebook's state before the refused downgrade is missing");
    }
    const busy = await downgradeNotebook(api, pat, notebook.id);
    expect([busy.response.status, busy.error?.code]).toEqual([409, "transfer.busy"]);
    await expectNotebook(db, before.data, adminId);
  } catch (error) {
    failed = true;
    throw error;
  } finally {
    // Release before waiting, and attempt every drain even if setup or a
    // prior assertion failed. A cleanup failure must not mask that error.
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

  await settleEvents(api, nervewiki.baseURL, pat, agents.id);
  const reader = await openEvents(readerPat);
  const changed = await downgradeNotebook(api, pat, notebook.id);
  expect(changed.response.status).toBe(200);
  if (!changed.data) {
    throw new Error("downgrade answered 200 without the notebook");
  }
  expect([changed.data.mode, changed.data.source_root_id]).toEqual(["standard", null]);
  await expectNotebook(db, changed.data, adminId);
  expect(await reader.expectNext("mode")).toEqual({ workspace_id: workspace.id, notebook_id: notebook.id });

  expect((await deleteNode(api, pat, raw.id)).response.status).toBe(204);
  await expectSubtreeDeleted(db, raw.id, [raw.id], adminId);
  expect((await deleteNode(api, pat, agents.id)).response.status).toBe(204);
  await expectSubtreeDeleted(db, agents.id, [agents.id], adminId);
  const forbidden = await downgradeNotebook(api, writerPat, notebook.id);
  expect([forbidden.response.status, forbidden.error?.code]).toEqual([403, "forbidden"]);
});

test("LW12 (page): only an admin can downgrade without typing a name; the same settings page in another account changes through its established stream, and AGENTS can then be deleted", async ({
  api,
  db,
  nervewiki,
  signedInPage,
  anotherPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes", "editor", { mode: "llm_wiki" });
  const writerTokens = await joinOnboarded(api, pat, workspace.slug, emailFor(testInfo, "writer"), "member");
  const agents = (await listNodes(api, pat, notebook.id)).find((node) => node.name === "AGENTS");
  if (!agents) {
    throw new Error("the LLM Wiki must have AGENTS");
  }
  await settleEvents(api, nervewiki.baseURL, pat, agents.id);
  const page = await signedInPage(tokens);
  const writer = await anotherPage(writerTokens);
  const streams = followStreams(writer.context());
  const notebookReads = countAnswers(writer, "GET", `/api/v0/workspaces/${workspace.slug}/notebooks`);
  const letStreamIn = await holdStream(writer);
  try {
    await page.goto(notebookPath(workspace.slug, notebook.id, "general"));
    await writer.goto(notebookPath(workspace.slug, notebook.id, "general"));
    const mode = page.locator("section", { has: page.getByRole("heading", { name: "Mode", exact: true }) });
    const otherMode = writer.locator("section", { has: writer.getByRole("heading", { name: "Mode", exact: true }) });
    const trigger = page.getByRole("button", { name: "Downgrade to a standard notebook…", exact: true });
    await expect(mode.getByText("LLM Wiki", { exact: true })).toBeVisible();
    await expect(trigger).toBeVisible();
    await expect(otherMode.getByText("LLM Wiki", { exact: true })).toBeVisible();
    await expect(writer.getByRole("button", { name: "Downgrade to a standard notebook…", exact: true })).toHaveCount(0);

    // Let the second context connect and finish its initial refresh before
    // changing mode. No navigation, focus or reconnect may stand in for mode.
    letStreamIn();
    await expect.poll(notebookReads).toBeGreaterThanOrEqual(2);
    await expect.poll(streams.count).toBe(1);
    const readsBefore = notebookReads();
    const streamsBefore = streams.opened();
    await writer.evaluate(() => Object.assign(window, { lw12Document: true }));

    await trigger.click();
    const dialog = page.getByRole("alertdialog", {
      name: `Downgrade “${notebook.name}” to a standard notebook?`,
      exact: true,
    });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("textbox")).toHaveCount(0);
    const answer = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/downgrade`);
    await downgradeWith(page);
    expect((await answer).status()).toBe(200);
    await expect(mode.getByText("Standard notebook", { exact: true })).toBeVisible();
    await expect(trigger).toHaveCount(0);
    await expect(mode.getByRole("heading", { name: "Mode", exact: true })).toBeFocused();

    await expect(otherMode.getByText("Standard notebook", { exact: true })).toBeVisible();
    expect(notebookReads()).toBeGreaterThan(readsBefore);
    expect(await writer.evaluate(() => "lw12Document" in window)).toBe(true);
    expect(streams.opened()).toBe(streamsBefore);
    expect(streams.count()).toBe(1);

    const tree = pageTree(page, notebook.name);
    await expect(tree.getByRole("link", { name: "AGENTS", exact: true })).toBeVisible();
    await tree.getByRole("button", { name: "Actions for AGENTS", exact: true }).click();
    const remove = page.getByRole("menuitem", { name: "Delete", exact: true });
    await expect(remove).toBeVisible();
    await remove.click();
    const deletion = page.getByRole("alertdialog", { name: "Delete AGENTS?", exact: true });
    const deleted = answerTo(page, "DELETE", `/api/v0/nodes/${agents.id}`);
    await deletion.getByRole("button", { name: "Delete", exact: true }).click();
    expect((await deleted).status()).toBe(204);
    await expect(tree.getByRole("link", { name: "AGENTS", exact: true })).toHaveCount(0);
    await expectSubtreeDeleted(db, agents.id, [agents.id], adminId);
  } finally {
    letStreamIn();
  }
});
