import { createHash } from "node:crypto";

import type { ApiClient, Page, Upgrade } from "@nervewiki/api-client";

import { accountIdOf } from "../../fixtures/assert/identity";
import { expectUpgrade } from "../../fixtures/assert/wiki";
import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { emailFor } from "../../fixtures/auth";
import { answerTo, countAnswers, failedToLoad } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { followStreams, holdStream } from "../../fixtures/events";
import { revertChangeset } from "../../fixtures/history";
import { joinAs, joinOnboarded } from "../../fixtures/invitations";
import { errorOf, type McpClient, mcpClient, resultOf } from "../../fixtures/mcp";
import { addedNotebookMember } from "../../fixtures/notebook-members";
import { notebookHeading, notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { postRevert } from "../../fixtures/transfer";
import {
  bannerText,
  letInAtHome,
  reviewDialog,
  upgradeBanner,
  upgradePath,
  upgradeSection,
} from "../../fixtures/upgrade-pages";
import { approveUpgrade, getUpgrade, postReject, startUpgrade, submitUpgrade } from "../../fixtures/upgrades";
import { changesetPath, pageTree, revertDialog, revertWith } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// UP7, an upgrade approved (M11/P3 design 5): a second admin approves the
// submitted upgrade of a notebook whose index at the root was deleted
// before it started, with a page and an attachment in raw and a note the
// agent gives its frontmatter. The notebook is an LLM Wiki whose source
// root is raw, its sources hashed as the SHA-256 of the bytes the test
// wrote, the upgrade's baseline gone; the
// deletion's revert, which would bring index back at the root, is
// page.virtual_file, in the request and in the background (decision 181).
// Through MCP, the agent lists the LLM Wiki's virtual index.md and
// log.md, and AGENTS is protected. On the page (S3), in the three
// browsers, the second admin approves it and lands on the notebook's home,
// the initiator's other tab following without a reload.

/** The story's notebook, submitted: its index deleted before, raw holding Paper and chart.png, Note at the root. */
async function submitted(
  api: ApiClient,
  db: Database,
  testInfo: Parameters<typeof newTeam>[1],
  baseURL: string,
  viaAgent: boolean
) {
  const { pat, workspace } = await newTeam(api, testInfo);
  const danaEmail = emailFor(testInfo, "dana");
  const dana = await joinAs(api, pat, workspace.slug, danaEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, danaEmail), "admin");
  const index = await createPage(api, pat, notebook.id, "index", null, "# Index\n");
  const gone = await deleteNode(api, pat, index.id);
  expect(gone.response.status).toBe(204);
  const deleted = await latestChangeset(db, notebook.id);
  const raw = await createPage(api, pat, notebook.id, "raw");
  await createPage(api, pat, notebook.id, "Paper", raw.id, "The paper.\n");
  await uploadAsset(api, pat, notebook.id, { name: "chart.png", bytes: pngBytes }, raw.id);
  const note = await createPage(api, pat, notebook.id, "Note", null, "# Note\n");
  await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(baseURL, pat, { notebook: notebook.id });
  await frontmatter(agent, note);
  if (viaAgent) {
    expect(resultOf<{ state: string }>(await agent.call("upgrade_submit", {}), "the agent's submission").state).toBe(
      "reviewing"
    );
  } else {
    await submitUpgrade(api, pat, notebook.id);
  }
  return { pat, dana, notebook, raw, deleted, agent };
}

/** submitted's for the page, its admin and dana onboarded: their tokens, for their pages; submitted through the API. */
async function submittedOnPage(api: ApiClient, db: Database, testInfo: Parameters<typeof newTeam>[1], baseURL: string) {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const danaEmail = emailFor(testInfo, "dana");
  const dana = await joinOnboarded(api, pat, workspace.slug, danaEmail, "member");
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  await addedNotebookMember(api, pat, notebook.id, await accountIdOf(db, danaEmail), "admin");
  const index = await createPage(api, pat, notebook.id, "index", null, "# Index\n");
  const gone = await deleteNode(api, pat, index.id);
  expect(gone.response.status).toBe(204);
  const deleted = await latestChangeset(db, notebook.id);
  const raw = await createPage(api, pat, notebook.id, "raw");
  await createPage(api, pat, notebook.id, "Paper", raw.id, "The paper.\n");
  await uploadAsset(api, pat, notebook.id, { name: "chart.png", bytes: pngBytes }, raw.id);
  const note = await createPage(api, pat, notebook.id, "Note", null, "# Note\n");
  await startUpgrade(api, pat, notebook.id);
  await frontmatter(await mcpClient(baseURL, pat, { notebook: notebook.id }), note);
  await submitUpgrade(api, pat, notebook.id);
  return { pat, tokens, dana, slug: workspace.slug, notebook, raw, deleted };
}

/** The agent gives Note its type and summary above its body. */
async function frontmatter(agent: McpClient, note: Page): Promise<void> {
  resultOf(
    await agent.call("write", {
      path: "Note.md",
      content: "---\ntype: note\nsummary: A note.\n---\n# Note\n",
      base_version: `${note.id}:${note.revision}`,
    }),
    "the agent's frontmatter of Note"
  );
}

/** The newest changeset of the notebook notebookId. */
async function latestChangeset(db: Database, notebookId: string): Promise<string> {
  const rows = await db.query<{ id: string }>(
    "SELECT id FROM changesets WHERE notebook_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1",
    [notebookId]
  );
  return rows[0]?.id ?? "";
}

/** The notebook notebookId's mode, its source root and its upgrade under way. */
async function modeOf(db: Database, notebookId: string) {
  return db.query("SELECT mode, source_root_id, upgrade_id FROM notebooks WHERE id = $1", [notebookId]);
}

/** The notebook notebookId's nodes with a source hash, each its name and the hash in hex. */
async function hashed(db: Database, notebookId: string) {
  return db.query(
    `SELECT name, encode(source_hash, 'hex') AS hash FROM nodes
      WHERE notebook_id = $1 AND source_hash IS NOT NULL ORDER BY name`,
    [notebookId]
  );
}

/** The SHA-256 of bytes, in hex. */
function sha256(bytes: string | Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex");
}

test("UP7 (API): a second admin approves the submitted upgrade; the notebook is an LLM Wiki whose source root is raw, its page and attachment hashed as the SHA-256 of the bytes written, the baseline gone and the upgrade read no more; reverting the deletion of index at the root is page.virtual_file, in the request and in the background", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, dana, notebook, raw, deleted } = await submitted(api, db, testInfo, nervewiki.baseURL, false);
  const before = await getUpgrade(api, pat, notebook.id);
  expect(before.data?.state).toBe("reviewing");

  const approved = await approveUpgrade(api, dana, notebook.id);
  expect(approved).toEqual({ ...before.data, state: "approved" });
  await expectUpgrade(db, approved);
  expect(await modeOf(db, notebook.id)).toEqual([{ mode: "llm_wiki", source_root_id: raw.id, upgrade_id: null }]);
  expect(await hashed(db, notebook.id)).toEqual([
    { name: "Paper", hash: sha256("The paper.\n") },
    { name: "chart.png", hash: sha256(pngBytes) },
  ]);
  expect(
    await db.query("SELECT count(*)::int AS rows FROM upgrade_nodes WHERE upgrade_id = $1", [approved.id])
  ).toEqual([{ rows: 0 }]);
  const read = await getUpgrade(api, pat, notebook.id);
  expect([read.response.status, read.error?.code]).toEqual([404, "wiki.upgrade_not_found"]);
  const late = await postReject(api, dana, notebook.id);
  expect([late.response.status, late.error?.code]).toEqual([409, "wiki.upgrade_state"]);

  const reverted = await revertChangeset(api, pat, deleted);
  expect([reverted.response.status, reverted.error?.code]).toEqual([409, "page.virtual_file"]);
  const background = await postRevert(api, pat, notebook.id, deleted);
  expect([background.response.status, background.error?.code]).toEqual([409, "page.virtual_file"]);
  expect(
    await db.query(
      "SELECT name, deleted_at IS NOT NULL AS trashed FROM nodes WHERE notebook_id = $1 AND name = 'index'",
      [notebook.id]
    )
  ).toEqual([{ name: "index", trashed: true }]);
});

test("UP7 (MCP): the agent submits through upgrade_submit and a second admin approves; then the agent's list of the root starts with the LLM Wiki's virtual index.md and log.md, and its delete of AGENTS.md is page.protected", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { dana, notebook, agent } = await submitted(api, db, testInfo, nervewiki.baseURL, true);
  await approveUpgrade(api, dana, notebook.id);
  const listing = resultOf<{ items: { path: string; kind: string; virtual?: boolean }[] }>(
    await agent.call("list", {}),
    "the agent's list of the root"
  );
  expect(listing.items.slice(0, 2)).toEqual([
    { path: "index.md", kind: "page", has_children: false, virtual: true },
    { path: "log.md", kind: "page", has_children: false, virtual: true },
  ]);
  expect(errorOf(await agent.call("delete", { path: "AGENTS.md" }), "the agent's delete of AGENTS").code).toBe(
    "page.protected"
  );
});

test("UP7 (page): a second admin approves the submitted upgrade on its page, its stream held: the page goes to the notebook's home, its title focused, the banner gone, the Directory in the tree; its stream let in there, its connection reads no upgrade; the initiator's tab on the home, its stream connected, loses the banner without a reload; reverting the deletion of index on its changeset's page is page.virtual_file, the dialog says so", async ({
  api,
  db,
  nervewiki,
  signedInPage,
  anotherPage,
  pageWatch,
}, testInfo) => {
  test.slow();
  const { pat, tokens, dana, slug, notebook, raw, deleted } = await submittedOnPage(
    api,
    db,
    testInfo,
    nervewiki.baseURL
  );
  const home = notebookPath(slug, notebook.id);

  // The initiator's tab on the notebook's home, its stream let in once it shows: from then on only the mode
  // event reads the notebooks again (as UP1).
  const tab = await anotherPage(tokens);
  const streams = followStreams(tab.context());
  const tabReads = countAnswers(tab, "GET", `/api/v0/workspaces/${slug}/notebooks`);
  const letTabIn = await holdStream(tab);
  try {
    await tab.goto(home);
    await expect(upgradeBanner(tab, bannerText.reviewing)).toBeVisible();
    letTabIn();
    await expect.poll(streams.count).toBe(1);
    await expect.poll(tabReads).toBeGreaterThanOrEqual(2);
  } finally {
    letTabIn();
  }
  await tab.evaluate(() => Object.assign(window, { up7Document: true }));

  // The approving page's stream is held until it is at the notebook's home: a mode event before the approval's answer
  // would read the upgrade, approved, again (M11/P3 design 5). Let in there, its connection reads what the home shows,
  // not the upgrade, its page unmounted; its reads all answered before the changeset's page loads, none cut short.
  const page = await signedInPage(dana);
  const before = await getUpgrade(api, pat, notebook.id);
  expect(before.data?.state).toBe("reviewing");
  const letStreamIn = await holdStream(page);
  try {
    await page.goto(upgradePath(slug, notebook.id));
    await expect(upgradeSection(page, "Check report")).toContainText("): every check passed.");
    await page.getByRole("button", { name: "Approve…", exact: true }).click();
    const dialog = reviewDialog(page, "Approve", notebook.name);
    await expect(dialog).toContainText("the pages and attachments under its source area (raw) are read-only");
    const approval = answerTo(page, "POST", `/api/v0/notebooks/${notebook.id}/upgrade/approve`);
    await dialog.getByRole("button", { name: "Approve the upgrade", exact: true }).click();
    const answer = await approval;
    expect(answer.status()).toBe(200);
    const approved = (await answer.json()) as Upgrade;
    expect(approved).toEqual({ ...before.data, state: "approved" });
    await expectUpgrade(db, approved);
    expect(await modeOf(db, notebook.id)).toEqual([{ mode: "llm_wiki", source_root_id: raw.id, upgrade_id: null }]);

    await expect(page).toHaveURL(home);
    await expect(notebookHeading(page, notebook.name)).toBeFocused();
    await expect(dialog).toHaveCount(0);
    await expect(pageTree(page, notebook.name).getByRole("link", { name: "Directory", exact: true })).toBeVisible();
    await expect(upgradeBanner(page, bannerText.reviewing)).toHaveCount(0);
    const connection = await letInAtHome(page, pageWatch, notebook.id, letStreamIn);
    expect(connection).toEqual(
      expect.arrayContaining(["GET /api/v0/workspaces", `GET /api/v0/notebooks/${notebook.id}/nodes`])
    );
    expect(connection).not.toContain(`GET /api/v0/notebooks/${notebook.id}/upgrade`);

    await expect(upgradeBanner(tab, bannerText.reviewing)).toHaveCount(0);
    await expect(pageTree(tab, notebook.name).getByRole("link", { name: "Directory", exact: true })).toBeVisible();
    expect(await tab.evaluate(() => "up7Document" in window)).toBe(true);

    // The deletion of index, reverted on its changeset's page, would bring it back at the root (decision 181).
    pageWatch.expectConsole({ errors: [failedToLoad(409)] });
    await page.goto(changesetPath(slug, notebook.id, deleted));
    const refused = await revertWith(page, deleted);
    expect([refused.status(), ((await refused.json()) as { code: string }).code]).toEqual([409, "page.virtual_file"]);
    await expect(revertDialog(page)).toContainText(
      "This file is generated by the system: the index updates itself as pages change, and the log is the notebook's changes. No page or attachment at the root may be named index or log."
    );
    expect(pageWatch.apiFailures).toEqual([`409 POST /api/v0/changesets/${deleted}/revert`]);
  } finally {
    letStreamIn();
  }
});
