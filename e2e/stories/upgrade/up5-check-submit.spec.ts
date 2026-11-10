import type { ApiClient, Page } from "@nervewiki/api-client";

import { expectUpgrade } from "../../fixtures/assert/wiki";
import { errorOf, type McpClient, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  checkUpgrade,
  getUpgrade,
  getUpgradeCheckItems,
  postSubmit,
  startUpgrade,
  submitUpgrade,
} from "../../fixtures/upgrades";
import { newTeam } from "../../fixtures/workspaces";

// UP5, the check and the submission (M11/P2 design 5): a notebook whose
// seven pages have no type nor summary is checked, its frontmatter gate
// not passed, its missing types read past the first five; its submission
// is wiki.upgrade_not_ready and leaves it upgrading. The initiator's agent
// gives each page its type and summary; checked again, every gate passes,
// the one wanted page as it started, the two pages the start wrote new; the
// submission keeps its report, and the notebook is frozen for everyone,
// the agent too. REST's writes are frozen while the upgrade is under way:
// the agent writes, through MCP (M11 design 13.4 #3). Through the API, then
// through MCP's upgrade_verify and upgrade_submit.

/** The seven pages of the story: Plan, which leads to a page that does not exist, and Note 1 to Note 6. */
async function sevenPages(api: ApiClient, pat: string, notebook: string): Promise<Page[]> {
  const pages = [await createPage(api, pat, notebook, "Plan", null, "See [[Missing]].\n")];
  for (const n of [1, 2, 3, 4, 5, 6]) {
    // oxlint-disable-next-line no-await-in-loop -- the pages in their order
    pages.push(await createPage(api, pat, notebook, `Note ${n}`, null, `Note ${n}.\n`));
  }
  return pages;
}

/** The agent gives each page its type and summary above its body, from the version REST answered. */
async function typed(agent: McpClient, pages: Page[]): Promise<void> {
  for (const page of pages) {
    // oxlint-disable-next-line no-await-in-loop -- one write at a time, as an agent does
    const answer = await agent.call("write", {
      path: `${page.name}.md`,
      content: `---\ntype: note\nsummary: ${page.name}.\n---\n${page.name === "Plan" ? "See [[Missing]].\n" : `${page.name}.\n`}`,
      base_version: `${page.id}:${page.revision}`,
    });
    resultOf(answer, `${page.name}'s type and summary`);
  }
}

test("UP5 (API): the check fails the frontmatter gate, seven missing types read past the first five; the submission is wiki.upgrade_not_ready, the upgrade upgrading; typed by the agent, the check passes, the submission keeps its report and freezes the agent", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const pages = await sevenPages(api, pat, notebook.id);
  await startUpgrade(api, pat, notebook.id);

  const first = await checkUpgrade(api, pat, notebook.id);
  expect([first.passed, first.gates.frontmatter.passed]).toEqual([false, false]);
  const missing = first.rules.find((rule) => rule.rule === "missing_type");
  expect([missing?.count, missing?.items.length, typeof missing?.next_cursor]).toEqual([7, 5, "string"]);
  const rest = await getUpgradeCheckItems(api, pat, notebook.id, "missing_type", {
    cursor: missing?.next_cursor ?? undefined,
  });
  expect(rest.response.status, JSON.stringify(rest.error)).toBe(200);
  expect([rest.data?.data.length, rest.data?.next_cursor]).toEqual([2, null]);
  const named = [...(missing?.items ?? []), ...(rest.data?.data ?? [])].map((item) =>
    "page_id" in item ? item.page_id : ""
  );
  expect(named.toSorted()).toEqual(pages.map((page) => page.id).toSorted());

  const refused = await postSubmit(api, pat, notebook.id);
  expect([refused.response.status, refused.error?.code]).toEqual([409, "wiki.upgrade_not_ready"]);
  expect(refused.error?.detail).toContain("frontmatter");
  const upgrading = await getUpgrade(api, pat, notebook.id);
  expect([upgrading.data?.state, upgrading.data?.submitted_by, upgrading.data?.report]).toEqual([
    "upgrading",
    null,
    null,
  ]);

  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  await typed(agent, pages);
  const second = await checkUpgrade(api, pat, notebook.id);
  expect([
    second.passed,
    second.counts.wanted_before.count,
    second.counts.wanted_now.count,
    second.counts.new_pages,
  ]).toEqual([true, 1, 1, 2]);

  const submitted = await submitUpgrade(api, pat, notebook.id);
  expect([submitted.state, submitted.report?.passed]).toEqual(["reviewing", true]);
  const read = await getUpgrade(api, pat, notebook.id);
  expect(read.data?.report).toEqual(submitted.report);
  await expectUpgrade(db, submitted);
  const frozen = await agent.call("edit", { path: "Note 1.md", old_string: "type: note", new_string: "type: plan" });
  expect(errorOf(frozen, "an edit in review").code).toBe("notebook.upgrading");
});

/** upgrade_verify's check, as much as the story reads. */
interface Verified {
  passed: boolean;
  gates: Record<string, { passed: boolean; paths?: (string | null)[] }>;
  rules: { rule: string; count: number; items: unknown[]; next_cursor: string | null }[];
}

test("UP5 (MCP): upgrade_verify answers the gates with their paths and a rule's items; upgrade_submit is wiki.upgrade_not_ready, then, typed, reviewing; in review the check still answers, passed, and the agent's edit is notebook.upgrading", async ({
  api,
  db,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const pages = await sevenPages(api, pat, notebook.id);
  await startUpgrade(api, pat, notebook.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  const first = resultOf<Verified>(await agent.call("upgrade_verify", {}), "the check");
  expect([first.passed, first.gates.frontmatter?.passed]).toEqual([false, false]);
  for (const gate of ["reserved_titles", "links", "preserved"]) {
    expect(first.gates[gate], gate).toEqual(expect.objectContaining({ passed: true, paths: [] }));
  }
  const missing = first.rules.find((rule) => rule.rule === "missing_type");
  expect([missing?.count, missing?.items.length]).toEqual([7, 5]);
  const one = resultOf<{ data: { page_id: string; path: string }[]; next_cursor: string | null }>(
    await agent.call("upgrade_verify", { rule: "missing_type", limit: 1 }),
    "a missing type"
  );
  expect([one.data.length, typeof one.next_cursor]).toEqual([1, "string"]);
  expect(pages.map((page) => `${page.name}.md`)).toContain(one.data[0]?.path);

  const refused = errorOf(await agent.call("upgrade_submit", {}), "the submission of an untyped notebook");
  expect(refused.code).toBe("wiki.upgrade_not_ready");
  expect(refused.message).toContain("frontmatter");

  await typed(agent, pages);
  const submitted = resultOf<{ state: string; submitted_at: string }>(
    await agent.call("upgrade_submit", {}),
    "the submission"
  );
  expect(submitted.state).toBe("reviewing");
  const reviewed = resultOf<Verified>(await agent.call("upgrade_verify", {}), "the check in review");
  expect(reviewed.passed).toBe(true);
  expect(
    errorOf(
      await agent.call("edit", { path: "Note 1.md", old_string: "type: note", new_string: "type: plan" }),
      "an edit in review"
    ).code
  ).toBe("notebook.upgrading");

  const read = await getUpgrade(api, pat, notebook.id);
  expect(read.data?.state).toBe("reviewing");
  expect(Date.parse(read.data?.submitted_at ?? "")).toBe(Date.parse(submitted.submitted_at));
  if (!read.data) {
    throw new Error("the upgrade in review read without it");
  }
  await expectUpgrade(db, read.data);
});
