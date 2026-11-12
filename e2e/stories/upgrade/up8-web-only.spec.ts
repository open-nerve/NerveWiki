import type { Upgrade, UpgradeCheck } from "@nervewiki/api-client";

import { expectUpgrade } from "../../fixtures/assert/wiki";
import { answerTo } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook, getNotebook } from "../../fixtures/notebooks";
import { listNodes } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  bannerText,
  modeSection,
  reviewDialog,
  startDialog,
  upgradeBanner,
  upgradePath,
  upgradeSection,
  upgradeTitle,
} from "../../fixtures/upgrade-pages";
import {
  approveUpgrade,
  checkUpgrade,
  getUpgrade,
  postApprove,
  startUpgrade,
  submitUpgrade,
} from "../../fixtures/upgrades";
import { pageTree } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// UP8, an upgrade carried through without an agent (M11/P3 design 5): an
// empty standard notebook's admin starts its upgrade, which writes raw and
// AGENTS, checks it, submits it and approves it, each answering as it
// should, the approval refused before the submission; approved, the
// notebook is an LLM Wiki whose root holds what the start wrote. Through
// the API, then on the pages (S3), in Chromium alone. No MCP version: the
// story is the one without an agent, and UP7 covers MCP after an approval.

test("UP8 (API): an empty notebook's admin starts, checks, submits and approves its upgrade with no agent; the notebook is an LLM Wiki whose root holds the raw and AGENTS the start wrote", async ({
  api,
  db,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");

  const started = await startUpgrade(api, pat, notebook.id);
  expect([started.state, started.seeded_page_ids.length, started.baseline.nodes]).toEqual(["upgrading", 2, 0]);
  const early = await postApprove(api, pat, notebook.id);
  expect([early.response.status, early.error?.code]).toEqual([409, "wiki.upgrade_state"]);
  const check = await checkUpgrade(api, pat, notebook.id);
  expect(check.passed).toBe(true);
  const submitted = await submitUpgrade(api, pat, notebook.id);
  expect([submitted.state, submitted.report?.passed]).toEqual(["reviewing", true]);
  const approved = await approveUpgrade(api, pat, notebook.id);
  expect(approved).toEqual({ ...submitted, state: "approved" });
  await expectUpgrade(db, approved);

  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.response.status, read.error?.code]).toEqual([404, "wiki.upgrade_not_found"]);
  const after = await getNotebook(api, pat, notebook.id);
  expect([after.data?.mode, after.data?.upgrade]).toEqual(["llm_wiki", null]);
  const nodes = await listNodes(api, pat, notebook.id);
  expect(nodes.map((node) => [node.name, node.parent_id, node.id]).toSorted()).toEqual(
    [
      ["AGENTS", null, started.seeded_page_ids[1]],
      ["raw", null, started.source_root_id],
    ].toSorted()
  );
});

test("UP8 (page): an empty notebook's admin starts the upgrade in its settings, checks it, submits it and approves it on its page, its stream held, with no agent; approved, the page goes to the notebook's home, its title focused, an LLM Wiki whose root holds the raw and AGENTS the start wrote", async ({
  api,
  db,
  signedInPage,
  pageWatch,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const upgradeRead = `/api/v0/notebooks/${notebook.id}/upgrade`;
  const page = await signedInPage(tokens);
  // The stream held throughout, as the approving page's in UP7: no mode event reads the upgrade, approved, again.
  const letStreamIn = await holdStream(page);
  let started: Upgrade | undefined;
  try {
    await page.goto(notebookPath(workspace.slug, notebook.id, "general"));
    await modeSection(page).getByRole("button", { name: "Upgrade to an LLM Wiki…", exact: true }).click();
    const start = answerTo(page, "POST", upgradeRead);
    await startDialog(page, notebook.name).getByRole("button", { name: "Start the upgrade", exact: true }).click();
    const startAnswer = await start;
    expect(startAnswer.status()).toBe(200);
    started = (await startAnswer.json()) as Upgrade;
    expect([started.state, started.seeded_page_ids.length, started.baseline.nodes]).toEqual(["upgrading", 2, 0]);
    await expect(page).toHaveURL(upgradePath(workspace.slug, notebook.id));
    await expect(upgradeTitle(page)).toBeFocused();
    await expect(upgradeBanner(page, bannerText.upgrading)).toBeVisible();

    const check = upgradeSection(page, "Check");
    const checked = answerTo(page, "GET", `${upgradeRead}/check`);
    await check.getByRole("button", { name: "Check", exact: true }).click();
    const checkAnswer = await checked;
    expect(checkAnswer.status()).toBe(200);
    expect(((await checkAnswer.json()) as UpgradeCheck).passed).toBe(true);
    await expect(check.getByRole("status", { name: "The check's result", exact: true })).toHaveText(
      /^Every check passed: the upgrade can be submitted for review\. Checked on .+\.$/
    );

    await page.getByRole("button", { name: "Submit for review…", exact: true }).click();
    const submitDialog = page.getByRole("alertdialog", { name: "Submit for review?", exact: true });
    const submission = answerTo(page, "POST", `${upgradeRead}/submit`);
    await submitDialog.getByRole("button", { name: "Submit for review", exact: true }).click();
    const submitAnswer = await submission;
    expect(submitAnswer.status()).toBe(200);
    const submitted = (await submitAnswer.json()) as Upgrade;
    expect([submitted.state, submitted.report?.passed]).toEqual(["reviewing", true]);
    await expect(submitDialog).toBeHidden();
    await expect(upgradeSection(page, "Check report")).toContainText("): every check passed.");
    await expect(upgradeBanner(page, bannerText.reviewing)).toBeVisible();

    await page.getByRole("button", { name: "Approve…", exact: true }).click();
    const approveDialog = reviewDialog(page, "Approve", notebook.name);
    const approval = answerTo(page, "POST", `${upgradeRead}/approve`);
    await approveDialog.getByRole("button", { name: "Approve the upgrade", exact: true }).click();
    const approveAnswer = await approval;
    expect(approveAnswer.status()).toBe(200);
    const approved = (await approveAnswer.json()) as Upgrade;
    expect(approved).toEqual({ ...submitted, state: "approved" });
    await expectUpgrade(db, approved);

    await expect(page).toHaveURL(notebookPath(workspace.slug, notebook.id));
    await expect(notebookHeading(page, notebook.name)).toBeFocused();
    await expect(upgradeBanner(page, bannerText.reviewing)).toHaveCount(0);
    await expect(pageTree(page, notebook.name).getByRole("link", { name: "Directory", exact: true })).toBeVisible();
  } finally {
    letStreamIn();
  }
  const after = await getNotebook(api, pat, notebook.id);
  expect([after.data?.mode, after.data?.upgrade]).toEqual(["llm_wiki", null]);
  const nodes = await listNodes(api, pat, notebook.id);
  expect(nodes.map((node) => [node.name, node.parent_id, node.id]).toSorted()).toEqual(
    [
      ["AGENTS", null, started?.seeded_page_ids[1]],
      ["raw", null, started?.source_root_id],
    ].toSorted()
  );
  expect(pageWatch.apiFailures).toEqual([]);
});
