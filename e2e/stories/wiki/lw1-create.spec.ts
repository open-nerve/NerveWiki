import type { ApiClient, Notebook } from "@nervewiki/api-client";

import { countNotebooks, expectNewNotebook } from "../../fixtures/assert/notebook";
import { expectChangesetOf, expectNewPage } from "../../fixtures/assert/page";
import { bearer } from "../../fixtures/auth";
import type { Database } from "../../fixtures/db";
import { listChangesets } from "../../fixtures/history";
import { mcpClient } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { listNodes, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

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
