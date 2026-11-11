import type { ApiClient, Page, Upgrade, UpgradeCheck } from "@nervewiki/api-client";

import { expectUpgrade } from "../../fixtures/assert/wiki";
import { answerTo, countAnswers, failedToLoad } from "../../fixtures/browser";
import { errorOf, type McpClient, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  bannerText,
  countsIn,
  upgradeBanner,
  upgradePath,
  upgradeSection,
  upgradeTitle,
} from "../../fixtures/upgrade-pages";
import {
  checkUpgrade,
  getUpgrade,
  getUpgradeCheckItems,
  postSubmit,
  startUpgrade,
  submitUpgrade,
} from "../../fixtures/upgrades";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// UP5, the check and the submission (M11/P2 design 5): a notebook whose
// seven pages have no type nor summary is checked, its frontmatter gate
// not passed, its missing types read past the first five; its submission
// is wiki.upgrade_not_ready and leaves it upgrading. The initiator's agent
// gives each page its type and summary; checked again, every gate passes,
// the one wanted page as it started, the two pages the start wrote new; the
// submission keeps its report, and the notebook is frozen for everyone,
// the agent too. REST's writes are frozen while the upgrade is under way:
// the agent writes, through MCP (M11 design 13.4 #3). Through the API, then
// through MCP's upgrade_verify and upgrade_submit, then on the upgrade's
// page (S3), in Chromium alone.

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

/** The rules in the order the page shows them, by their titles. */
const ruleTitles = [
  ["wanted", "Wanted pages"],
  ["orphans", "Orphan pages"],
  ["ambiguous", "Ambiguous links"],
  ["invalid_frontmatter", "Invalid frontmatter"],
  ["missing_type", "Missing type"],
  ["missing_summary", "Missing summary"],
  ["undigested_sources", "Undigested sources"],
  ["invalid_sources", "Invalid sources"],
] as const;

/** What the page says of a count, capped with a +. */
const counted = ({ count, count_capped: capped }: { count: number; count_capped: boolean }) =>
  `${count.toLocaleString("en-US")}${capped ? "+" : ""}`;

/** The counts of a check or a report as the page lists them, each term with its count. */
function countsOf(counts: UpgradeCheck["counts"]): string[][] {
  return [
    ["Nodes in the source area", counted({ count: counts.sources, count_capped: false })],
    [
      "Nodes it started with, moved into the source area",
      counted({ count: counts.moved_into_sources, count_capped: false }),
    ],
    ["Pages outside the source area", counted({ count: counts.wiki_pages, count_capped: false })],
    ["Pages created since the upgrade started", counted({ count: counts.new_pages, count_capped: false })],
    ["Changes of the upgrade", counted({ count: counts.changesets, count_capped: false })],
    ["Wanted pages as the upgrade started", counted(counts.wanted_before)],
    ["Wanted pages now", counted(counts.wanted_now)],
  ];
}

test("UP5 (page): the admin checks the upgrade, the frontmatter gate not passed, the missing types' card read past its first five with More; the submission is not ready, the dialog says so and the check is read again, a new one; closed, the focus is on Submit for review…; typed by the agent, checked again, every check passes; submitted, the dialog closes, the focus is on the title, the page shows the report kept and the banner says the upgrade waits for review", async ({
  api,
  db,
  nervewiki,
  signedInPage,
  pageWatch,
}, testInfo) => {
  test.slow();
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Field notes");
  const pages = await sevenPages(api, pat, notebook.id);
  await startUpgrade(api, pat, notebook.id);
  const pathOf = (id: string) => `${pages.find((each) => each.id === id)?.name ?? id}.md`;
  const checkPath = `/api/v0/notebooks/${notebook.id}/upgrade/check`;
  const submitPath = `/api/v0/notebooks/${notebook.id}/upgrade/submit`;
  // The submission not ready is the story's: the browser logs its answer of 409.
  pageWatch.expectConsole({ errors: [failedToLoad(409)] });
  const page = await signedInPage(tokens);
  const checks = countAnswers(page, "GET", checkPath);
  await page.goto(upgradePath(workspace.slug, notebook.id));
  await expect(upgradeBanner(page, bannerText.upgrading)).toBeVisible();
  const check = upgradeSection(page, "Check");
  const result = check.getByRole("status", { name: "The check's result", exact: true });
  const missing = check.locator("section", {
    has: page.getByRole("heading", { level: 3, name: "Missing type", exact: true }),
  });
  const submit = page.getByRole("button", { name: "Submit for review…", exact: true });
  const dialog = page.getByRole("alertdialog", { name: "Submit for review?", exact: true });
  const confirm = dialog.getByRole("button", { name: "Submit for review", exact: true });

  // Checked, the frontmatter gate is not passed; the missing types' card
  // shows the first five, More the other two.
  const firstAnswer = answerTo(page, "GET", checkPath);
  await check.getByRole("button", { name: "Check", exact: true }).click();
  const first = (await (await firstAnswer).json()) as UpgradeCheck;
  expect(Object.entries(first.gates).flatMap(([gate, { passed }]) => (passed ? [] : [gate]))).toEqual(["frontmatter"]);
  await expect(result).toHaveText(/^1 check did not pass\. Checked on .+\.$/);
  await expect(check.getByRole("list", { name: "The checks", exact: true }).locator(":scope > li > p")).toContainText([
    "Each page outside the source area, the source root and the root's AGENTS included, has frontmatter that parses; each except these two has a type and a summary: not passed",
  ]);
  expect(await countsIn(check)).toEqual(countsOf(first.counts));
  const firstTypes = first.rules.find((rule) => rule.rule === "missing_type");
  expect([firstTypes?.count, firstTypes?.items.length]).toEqual([7, 5]);
  const firstFive = (firstTypes?.items ?? []).map((item) => pathOf("page_id" in item ? item.page_id : ""));
  await expect(missing.getByRole("listitem")).toHaveText(firstFive);
  const moreAnswer = answerTo(page, "GET", `${checkPath}/missing_type`);
  await missing.getByRole("button", { name: "More", exact: true }).click();
  const more = (await (await moreAnswer).json()) as { data: { page_id: string }[] };
  const allSeven = [...firstFive, ...more.data.map((item) => pathOf(item.page_id))];
  await expect(missing.getByRole("listitem")).toHaveText(allSeven);
  expect(allSeven.toSorted()).toEqual(pages.map((each) => `${each.name}.md`).toSorted());
  await expect(missing.getByRole("button", { name: "More", exact: true })).toHaveCount(0);

  // Submitted, the upgrade is not ready: upgrading again, read again, and
  // its check read again, a new one, while the dialog says so (the page
  // may show the upgrade reviewing as the server checks it, decision 264).
  await submit.click();
  const refusal = answerTo(page, "POST", submitPath);
  const recheck = answerTo(page, "GET", checkPath);
  await confirm.click();
  expect((await refusal).status()).toBe(409);
  await expect(dialog).toContainText(
    "The check did not pass, and the upgrade is upgrading again. The check below is read again."
  );
  const second = (await (await recheck).json()) as UpgradeCheck;
  expect([second.passed, second.gates.frontmatter.passed]).toEqual([false, false]);
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(submit).toBeFocused();
  await expect(result).toHaveText(/^1 check did not pass\. Checked on .+\.$/);
  await expect(missing.getByRole("listitem")).toHaveCount(5);
  await expect(missing.getByRole("button", { name: "More", exact: true })).toBeVisible();
  expect(checks()).toBe(2);

  // The agent types the pages; checked again, every check passes.
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });
  await typed(agent, pages);
  const thirdAnswer = answerTo(page, "GET", checkPath);
  await check.getByRole("button", { name: "Check again", exact: true }).click();
  const third = (await (await thirdAnswer).json()) as UpgradeCheck;
  expect(third.passed).toBe(true);
  await expect(result).toHaveText(/^Every check passed: the upgrade can be submitted for review\. Checked on .+\.$/);
  await expect(check.getByRole("list", { name: "Missing type", exact: true })).toHaveCount(0);

  // Submitted, it keeps its report, shown in review.
  await submit.click();
  const stored = answerTo(page, "POST", submitPath);
  await confirm.click();
  const answer = await stored;
  expect(answer.status()).toBe(200);
  const submitted = (await answer.json()) as Upgrade;
  expect([submitted.state, submitted.report?.passed]).toEqual(["reviewing", true]);
  await expect(dialog).toBeHidden();
  await expect(upgradeTitle(page)).toBeFocused();
  await expect(upgradeBanner(page, bannerText.reviewing)).toBeVisible();
  const report = upgradeSection(page, "Check report");
  await expect(report).toContainText("The check at its submission (");
  await expect(report).toContainText("): every check passed.");
  await expect(
    report.getByRole("list", { name: "The checks", exact: true }).locator(":scope > li > p:first-child")
  ).toHaveText(Array.from({ length: 8 }, () => /: passed$/));
  expect(await countsIn(report)).toEqual(countsOf(submitted.report?.counts ?? first.counts));
  const ruleLines = await report
    .locator("section")
    .evaluateAll((sections) =>
      sections.map((section) => [section.querySelector("h3")?.textContent, section.querySelector("p")?.textContent])
    );
  expect(ruleLines).toEqual(
    ruleTitles.map(([rule, title]) => {
      const found = submitted.report?.rules.find((each) => each.rule === rule);
      const issues = found === undefined ? "" : counted(found);
      return [title, found?.count === 1 && !found.count_capped ? "1 issue" : `${issues} issues`];
    })
  );
  await expect(report.getByRole("button")).toHaveCount(0);
  await expect(check).toHaveCount(0);
  await expect(submit).toHaveCount(0);
  const read = await getUpgrade(api, pat, notebook.id);
  expect(read.data?.report).toEqual(submitted.report);
  await expectUpgrade(db, submitted);
  expect(checks()).toBe(3);
  expect(pageWatch.apiFailures).toEqual([`409 POST ${submitPath}`]);
});
