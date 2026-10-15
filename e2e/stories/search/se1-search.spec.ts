import type { ApiClient, SearchHit, SearchPage } from "@nervewiki/api-client";
import type { Page } from "@playwright/test";

import { uploadAsset, utf8 } from "../../fixtures/assets";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, deleteNode } from "../../fixtures/pages";
import { search } from "../../fixtures/search";
import { expect, test } from "../../fixtures/test";
import { restoreNode } from "../../fixtures/trash";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { resultNames, searchFor, searchResults, searchStatus } from "../../fixtures/wiki-search";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// SE1, full-text search (M8 design 4.12, 4.13, section 9; M8/P5 design
// 4.6): a notebook's pages are found by their titles and contents, its
// attachments by their names, a hit's terms marked. Two CJK characters
// and more, and three letters and more, are found as substrings ("api" in
// "rapid"), one or two letters as a whole word ("ai" is not in "said"), a
// single CJK character in titles alone ("库" is not the content's "仓库").
// A page deleted is found no more at once, and again once restored. The
// API's version and the page's check the same finds. No assertion of the
// database: a search writes nothing, and the job that builds the index
// may or may not have run (a write a user makes is found either way).

/** The bytes of a PDF, which the server takes for one. */
const pdfBytes = utf8("%PDF-1.4\n% a quarterly report\n");

/**
 * Writes the notebook SE1 searches, in the workspace of slug, with credential: a page titled 数据库设计; Storage,
 * whose content holds 全文检索, Go语言 and 仓库, its title no 库, with the attachment 季度报告.pdf; Tools, whose content
 * holds "rapid" and "AI" alone; and Quotes, whose content holds "said" alone.
 */
async function written(api: ApiClient, credential: string, slug: string) {
  const notebook = await createNotebook(api, credential, slug, "Plans");
  await createPage(api, credential, notebook.id, "数据库设计", null, "表与索引。\n");
  const storage = await createPage(
    api,
    credential,
    notebook.id,
    "Storage",
    null,
    "我们用Go语言写服务端，全文检索的代码放在仓库里。\n"
  );
  await createPage(api, credential, notebook.id, "Tools", null, "A rapid prototype uses AI to sort pages.\n");
  await createPage(api, credential, notebook.id, "Quotes", null, "She said it works.\n");
  const report = await uploadAsset(api, credential, notebook.id, { name: "季度报告.pdf", bytes: pdfBytes }, storage.id);
  return { notebook, storage, report };
}

/** What a search found: its hits' names in order, what the first hit marks in its name and its excerpt, and how many there are by an exact count. */
interface Found {
  names: string[];
  marked: string[];
  count: number | undefined;
}

/** What each of SE1's queries finds: the hits' names, and what the first marks. */
const finds: { q: string; names: string[]; marked: string[] }[] = [
  // Two CJK characters, four, and a term of letters and CJK.
  { q: "检索", names: ["Storage"], marked: ["检索"] },
  { q: "全文检索", names: ["Storage"], marked: ["全文检索"] },
  { q: "Go语言", names: ["Storage"], marked: ["Go语言"] },
  // Three letters in a word; two as a whole word, not in "said".
  { q: "api", names: ["Tools"], marked: ["api"] },
  { q: "ai", names: ["Tools"], marked: ["AI"] },
  // A title; a single CJK character, in titles alone: not the content's 仓库.
  { q: "数据库", names: ["数据库设计"], marked: ["数据库"] },
  { q: "库", names: ["数据库设计"], marked: ["库"] },
  // An attachment by its name.
  { q: "季度报告", names: ["季度报告.pdf"], marked: ["季度报告"] },
];

/** expectFound checks what the search of q found against what SE1 expects of it. */
function expectFound(q: string, found: Found): void {
  const expected = finds.find((each) => each.q === q);
  expect(found, q).toEqual({ names: expected?.names, marked: expected?.marked, count: expected?.names.length });
}

/** marks are the parts of text the ranges name. */
function marks(text: string, ranges: { start: number; end: number }[]): string[] {
  return ranges.map(({ start, end }) => text.slice(start, end));
}

/** foundOf is what the API's page of a search found. */
function foundOf(page: SearchPage | undefined): Found {
  const first: SearchHit | undefined = page?.data[0];
  return {
    names: page?.data.map((hit) => hit.node.name) ?? [],
    marked:
      first === undefined
        ? []
        : [
            ...marks(first.node.name, first.title_hits),
            ...(first.excerpt === null ? [] : marks(first.excerpt.text, first.excerpt.hits)),
          ],
    count: page?.count?.kind === "exact" ? page.count.value : undefined,
  };
}

/** shownOf is what the results page on page shows: the hits' names, the first's marks, and the count its line says. */
async function shownOf(page: Page): Promise<Found> {
  const status = (await searchStatus(page).textContent()) ?? "";
  const count = status === "Nothing found." ? 0 : /^Results: (\d+)\.$/.exec(status)?.[1];
  return {
    names: await resultNames(page),
    marked:
      (await searchResults(page).count()) === 0
        ? []
        : await searchResults(page).first().locator("mark").allTextContents(),
    count: count === undefined ? undefined : Number(count),
  };
}

test("SE1 (API): pages are found by their titles and contents, an attachment by its name, the hits' terms marked, counted exactly; CJK from two characters and letters from three as substrings, two letters as a whole word, one CJK character in titles alone; a page deleted is found no more, and again once restored", async ({
  api,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const { notebook, storage, report } = await written(api, pat, workspace.slug);

  for (const { q } of finds) {
    // oxlint-disable-next-line no-await-in-loop -- a query at a time
    const { data, error, response } = await search(api, pat, workspace.slug, q);
    expect(response.status, `${q}: ${JSON.stringify(error)}`).toBe(200);
    expectFound(q, foundOf(data));
    expect(data?.next_cursor, q).toBeNull();
    expect(data?.building, q).toBe(false);
  }

  // A page's hit: where it is, when it changed, its content around the hit.
  const stored = (await search(api, pat, workspace.slug, "检索")).data?.data[0];
  expect(stored).toEqual({
    node: { id: storage.id, notebook_id: notebook.id, kind: "page", name: "Storage" },
    ancestors: [],
    title_hits: [],
    excerpt: {
      text: expect.stringMatching(/^我们用Go语言写服务端，全文检索的代码放在仓库里。\s*$/),
      hits: [{ start: 14, end: 16 }],
      cut_start: false,
      cut_end: false,
    },
    changed_at: expect.any(String),
    attachment: null,
  });
  // An attachment's: under its page, its type and size, no excerpt.
  const filed = (await search(api, pat, workspace.slug, "季度报告")).data?.data[0];
  expect(filed).toEqual({
    node: { id: report.id, notebook_id: notebook.id, kind: "asset", name: "季度报告.pdf" },
    ancestors: [{ id: storage.id, name: "Storage" }],
    title_hits: [{ start: 0, end: 4 }],
    excerpt: null,
    changed_at: expect.any(String),
    attachment: { mime: "application/pdf", byte_size: pdfBytes.length },
  });

  // Deleted, found no more at once; restored, found again at once.
  expect((await deleteNode(api, pat, storage.id)).response.status).toBe(204);
  expect(foundOf((await search(api, pat, workspace.slug, "检索")).data)).toEqual({ names: [], marked: [], count: 0 });
  expect((await restoreNode(api, pat, storage.id)).response.status).toBe(200);
  expectFound("检索", foundOf((await search(api, pat, workspace.slug, "检索")).data));
});

test("SE1 (page): searched from the left column's box, each find is listed, its terms marked and its count said, the title taking the focus; an attachment found opens on its page, its row taking the focus; a page deleted is listed no more as the same tab searches again, and again once restored", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const { notebook, storage, report } = await written(api, pat, workspace.slug);
  const page = await signedInPage(tokens);
  const main = page.getByRole("main");
  await page.goto(wikiPagePath(workspace.slug, notebook.id, storage.id));
  await expect(pageHeading(page, "Storage")).toBeVisible();
  // The tab is not loaded again: what the page holds stays.
  await page.evaluate(() => {
    (window as unknown as { se1: boolean }).se1 = true;
  });

  // Submitted from a page of the notebook: the results page's title takes the focus, its line describing it.
  expect((await searchFor(page, "检索")).status()).toBe(200);
  const title = main.getByRole("heading", { level: 1, name: "Search “检索”", exact: true });
  await expect(title).toBeFocused();
  await expect(page).toHaveTitle("Search: 检索 · Acme · Nerve Wiki");
  await expect(searchStatus(page)).toHaveText("Results: 1.");
  expectFound("检索", await shownOf(page));

  for (const { q } of finds.slice(1)) {
    // oxlint-disable-next-line no-await-in-loop -- a query at a time, each in the box
    expect((await searchFor(page, q)).status(), q).toBe(200);
    // oxlint-disable-next-line no-await-in-loop -- its line said
    await expect(main.getByRole("heading", { level: 1, name: `Search “${q}”`, exact: true })).toBeVisible();
    // oxlint-disable-next-line no-await-in-loop -- its hits listed
    await expect(searchStatus(page)).toHaveText(/^Results: \d+\.$/);
    // oxlint-disable-next-line no-await-in-loop -- what it shows
    expectFound(q, await shownOf(page));
  }

  // The attachment, on the page it is under: its row has the focus.
  await searchResults(page).getByRole("link", { name: "季度报告.pdf", exact: true }).click();
  await expect(pageHeading(page, "Storage")).toBeVisible();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, storage.id));
  await expect(page.locator(`[data-attachment="${report.id}"]`)).toBeFocused();

  // Deleted in another tab: the same tab's search, asked again, lists it no more; restored, it is back.
  expect((await searchFor(page, "检索")).status()).toBe(200);
  await expect(searchStatus(page)).toHaveText("Results: 1.");
  expect((await deleteNode(api, pat, storage.id)).response.status).toBe(204);
  expect((await searchFor(page, "检索")).status()).toBe(200);
  await expect(searchStatus(page)).toHaveText("Nothing found.");
  await expect(searchResults(page)).toHaveCount(0);
  expect((await restoreNode(api, pat, storage.id)).response.status).toBe(200);
  expect((await searchFor(page, "检索")).status()).toBe(200);
  await expect(searchStatus(page)).toHaveText("Results: 1.");
  expectFound("检索", await shownOf(page));
  expect(await page.evaluate(() => (window as unknown as { se1?: boolean }).se1)).toBe(true);
});
