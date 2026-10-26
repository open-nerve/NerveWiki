import type { ApiClient, Notebook } from "@nervewiki/api-client";

import { countNotebooks, expectNewNotebook } from "../../fixtures/assert/notebook";
import { expectChangesetOf, expectNewPage } from "../../fixtures/assert/page";
import { bearer } from "../../fixtures/auth";
import { answerTo } from "../../fixtures/browser";
import type { Database } from "../../fixtures/db";
import { listChangesets } from "../../fixtures/history";
import { mcpClient } from "../../fixtures/mcp";
import { createNotebookWith, notebookHeading, notebookPath, workspaceNav } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { listNodes, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageTree } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// LW1, creating an LLM Wiki (M10 design 4.1; M10/P1 design 5.2): the
// notebook and its template, raw and AGENTS, in one change. The e2e cannot
// import the server's templates: an AGENTS's first line tells its
// language (M10/P1 design 3.9).

/** Each template's AGENTS's first line, and its changeset's message, by language. */
const templates = {
  "zh-CN": { heading: "# 这本 LLM Wiki 的约定", message: "LLM Wiki 模板" },
  en: { heading: "# How this LLM Wiki is kept", message: "LLM Wiki template" },
} as const;

/**
 * created's root holds raw, its source root, then AGENTS of locale's template: pages adminId wrote through the API at
 * the notebook's time, in its one changeset.
 */
async function expectTemplate(
  api: ApiClient,
  db: Database,
  pat: string,
  adminId: string,
  created: Notebook,
  locale: keyof typeof templates
): Promise<void> {
  const nodes = await listNodes(api, pat, created.id);
  expect(nodes.map((n) => [n.name, n.kind, n.parent_id])).toEqual([
    ["raw", "page", null],
    ["AGENTS", "page", null],
  ]);
  expect(nodes[0]?.id).toBe(created.source_root_id);
  const contents = await Promise.all(
    nodes.map(async (node) => {
      const { content } = await readContent(api, pat, node.id);
      await expectNewPage(db, { ...node, created_at: created.created_at }, adminId, "api", content);
      return content;
    })
  );
  expect(contents[1]?.split("\n")[0], `${created.name}'s AGENTS`).toBe(templates[locale].heading);
  const log = await listChangesets(api, pat, created.id);
  expect(log.response.status).toBe(200);
  expect(log.data?.data).toHaveLength(1);
  await expectChangesetOf(db, log.data?.data[0]?.id ?? "", {
    client: "api",
    byId: adminId,
    items: nodes.map((n) => n.id),
    versions: nodes.map((n) => n.id),
    message: templates[locale].message,
    kind: "edit",
  });
}

test("LW1 (API): an LLM Wiki is created with its template in the language asked, en when none is, a standard notebook with nothing; a language there is no template of is refused", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);

  const wikis = [
    ["Field notes", "zh-CN", "zh-CN"],
    ["Lab notes", "en", "en"],
    ["Reading", undefined, "en"],
  ] as const;
  await Promise.all(
    wikis.map(async ([name, templateLocale, locale]) => {
      const created = await createNotebook(api, pat, workspace.slug, name, undefined, {
        mode: "llm_wiki",
        templateLocale,
      });
      expect(created).toMatchObject({ name, mode: "llm_wiki", role: "admin" });
      await expectNewNotebook(db, created, adminId);
      await expectTemplate(api, db, pat, adminId, created, locale);
    })
  );

  const standard = await createNotebook(api, pat, workspace.slug, "Plain");
  expect(standard).toMatchObject({ mode: "standard", source_root_id: null });
  await expectNewNotebook(db, standard, adminId);
  expect(await listNodes(api, pat, standard.id)).toEqual([]);

  // The template's language is checked, as a value, after the decision;
  // nothing is added.
  const before = await countNotebooks(db);
  const refused = await api.POST("/api/v0/workspaces/{slug}/notebooks", {
    params: { path: { slug: workspace.slug } },
    // @ts-expect-error -- the request asks for a language there is no template of on purpose
    body: { name: "Notes de terrain", mode: "llm_wiki", template_locale: "fr" },
    headers: bearer(pat),
  });
  expect(refused.response.status).toBe(422);
  expect(refused.error?.errors?.map((e) => [e.field, e.code])).toEqual([["template_locale", "invalid_format"]]);
  expect(await countNotebooks(db)).toEqual(before);
});

test("LW1 (MCP): an agent of an LLM Wiki is given its AGENTS page as its instructions, and sees the notebook's mode", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const created = await createNotebook(api, pat, workspace.slug, "Field notes", undefined, {
    mode: "llm_wiki",
    templateLocale: "en",
  });
  const agents = (await listNodes(api, pat, created.id)).find((n) => n.name === "AGENTS");
  if (!agents) {
    throw new Error("the LLM Wiki has no AGENTS page at its root");
  }
  const { content } = await readContent(api, pat, agents.id);

  // The notebook entry's handshake: the AGENTS page's text.
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: created.id });
  expect(agent.instructions).toBe(content.trim());

  // The account entry: the notebook's mode in the list and when opened.
  const account = await mcpClient(nervewiki.baseURL, pat);
  const listed = await account.call("notebooks");
  expect(listed.isError, listed.text.join(" ")).toBe(false);
  const notebooks = (listed.structured as { notebooks: { id: string; mode: string }[] }).notebooks;
  expect(notebooks.find((n) => n.id === created.id)?.mode).toBe("llm_wiki");
  const opened = await account.call("open_notebook", { notebook: created.id });
  expect(opened.isError, opened.text.join(" ")).toBe(false);
  expect(opened.structured).toMatchObject({ notebook: created.id, mode: "llm_wiki", instructions_from: "AGENTS.md" });
});

test("LW1 (page): an English LLM Wiki starts with its source area and instructions, named in the tree", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { adminId, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const page = await signedInPage(tokens);
  // Pass-through interception lets Chromium expose this POST's body to Playwright; install it before navigation.
  await page.route(`**/api/v0/workspaces/${workspace.slug}/notebooks`, (route) => route.continue());
  await page.goto(`/${workspace.slug}`);

  const answer = answerTo(page, "POST", `/api/v0/workspaces/${workspace.slug}/notebooks`);
  const { status, created } = await createNotebookWith(page, workspace, "Field notes", undefined, "llm_wiki");
  expect(status).toBe(201);
  expect((await answer).request().postDataJSON()).toEqual({
    name: "Field notes",
    workspace_access: "none",
    mode: "llm_wiki",
    template_locale: "en",
  });
  expect(created.mode).toBe("llm_wiki");
  await expectNewNotebook(db, created, adminId);
  await expect(page).toHaveURL(notebookPath(workspace.slug, created.id));
  await expect(notebookHeading(page, created.name)).toBeFocused();

  const tree = pageTree(page, created.name);
  await expect(tree.getByRole("link", { name: "raw (source area)", exact: true })).toBeVisible();
  const agents = tree.getByRole("link", { name: "AGENTS (instructions for agents)", exact: true });
  await expect(agents).toBeVisible();
  await agents.click();
  await expect(
    page.getByRole("article", { name: "AGENTS", exact: true }).getByRole("heading", {
      name: "How this LLM Wiki is kept",
      exact: true,
    })
  ).toBeVisible();
});

test.describe("the Chinese creation page", () => {
  test.use({ locale: "zh-CN" });

  test("LW1 (page): a Chinese LLM Wiki starts with Chinese instructions and its template's history", async ({
    api,
    db,
    signedInPage,
  }, testInfo) => {
    const { adminId, tokens, workspace } = await newOnboardedTeam(api, testInfo);
    const page = await signedInPage(tokens);
    // As in the English story, observe the request body without replacing the request or its answer.
    await page.route(`**/api/v0/workspaces/${workspace.slug}/notebooks`, (route) => route.continue());
    await page.goto(`/${workspace.slug}`);

    await workspaceNav(page, workspace.name).getByRole("button", { name: "新建笔记本", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "新建笔记本", exact: true });
    await dialog.getByLabel("名称", { exact: true }).fill("研究笔记");
    await dialog.getByRole("radio", { name: "LLM Wiki", exact: true }).check();
    const answer = answerTo(page, "POST", `/api/v0/workspaces/${workspace.slug}/notebooks`);
    await dialog.getByRole("button", { name: "新建", exact: true }).click();
    const response = await answer;
    expect(response.status()).toBe(201);
    expect(response.request().postDataJSON()).toEqual({
      name: "研究笔记",
      workspace_access: "none",
      mode: "llm_wiki",
      template_locale: "zh-CN",
    });
    const created = (await response.json()) as Notebook;
    expect(created.mode).toBe("llm_wiki");
    await expectNewNotebook(db, created, adminId);
    await expect(page).toHaveURL(notebookPath(workspace.slug, created.id));
    await expect(notebookHeading(page, created.name)).toBeFocused();

    const tree = page.getByRole("navigation", { name: "研究笔记的页面", exact: true });
    await expect(tree.getByRole("link", { name: "raw （来源区）", exact: true })).toBeVisible();
    const agents = tree.getByRole("link", { name: "AGENTS （给 agent 的约定）", exact: true });
    await expect(agents).toBeVisible();
    await agents.click();
    await expect(
      page.getByRole("article", { name: "AGENTS", exact: true }).getByRole("heading", {
        name: "这本 LLM Wiki 的约定",
        exact: true,
      })
    ).toBeVisible();

    await page
      .getByRole("navigation", { name: "研究笔记的更多", exact: true })
      .getByRole("link", { name: "历史", exact: true })
      .click();
    const changes = page.getByRole("main").getByRole("list", { name: "改动", exact: true }).getByRole("listitem");
    await expect(changes).toHaveCount(1);
    await expect(changes.getByText("LLM Wiki 模板", { exact: true })).toBeVisible();
    await expect(changes).toContainText("网页 · ");
  });
});
