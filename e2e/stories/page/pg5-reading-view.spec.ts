import { expectContentWritten } from "../../fixtures/assert/page";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG5, reading a page (M4 design 3; M4/P4 design 3.12): the reading view of
// a page created with its Markdown, and in the browser its code coloured by
// a worker of the app's own, which loads nothing (M4/P5 design 3.8, 3.9);
// writing it comes with the editor (M4/P6).

test("PG5 (API): a page created with Markdown reads as HTML, its properties first, then its table, task items, strikethrough, autolink, footnote and code in its language; the frontmatter is neither a rule nor a heading", async ({
  api,
  db,
}, testInfo) => {
  const { adminId, pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const content = [
    "---",
    "status: draft",
    "owner: ada",
    "---",
    "# Q4",
    "",
    "| a | b |",
    "|:--|:-:|",
    "| 1 | 2 |",
    "",
    "- [ ] open",
    "- [x] done",
    "",
    "~~gone~~, <https://example.com/a> and a note[^1].",
    "",
    "```go",
    "a < b",
    "```",
    "",
    "[^1]: The note.",
    "",
  ].join("\n");
  const page = await createPage(api, pat, notebook.id, "Q4", null, content);
  expect(page).toMatchObject({ revision: 1, byte_size: Buffer.byteLength(content) });
  await expectContentWritten(db, page, content, adminId);

  const { data, error, response } = await getView(api, pat, page.id);
  expect(response.status, JSON.stringify(error)).toBe(200);
  expect(data?.revision).toBe(1);
  const html = data?.html ?? "";
  expect(
    html.startsWith(
      '<table class="nw-props"><tr><th>status</th><td>draft</td></tr><tr><th>owner</th><td>ada</td></tr></table>'
    )
  ).toBe(true);
  for (const element of [
    '<h1 id="nw-q4">Q4</h1>',
    '<th align="left">a</th>',
    '<input disabled="" type="checkbox"> open',
    '<input checked="" disabled="" type="checkbox"> done',
    "<del>gone</del>",
    '<a href="https://example.com/a">https://example.com/a</a>',
    'class="footnote-ref"',
    '<div class="footnotes" role="doc-endnotes">',
    '<pre><code class="language-go">a &lt; b\n</code></pre>',
  ]) {
    expect(html).toContain(element);
  }
  // The frontmatter's fences are no thematic break and no setext heading:
  // the one rule is the footnotes'.
  expect(html.match(/<hr>/g)).toHaveLength(1);
  expect(html).not.toContain("<h2");
});

test("PG5 (page): the reading view shows the page's properties first, then its heading, table, task items and footnote; its code is coloured by a worker of the app's own origin, and nothing breaks the CSP", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const content = [
    "---",
    "status: draft",
    "---",
    "# Q4",
    "",
    "| a | b |",
    "|:--|:-:|",
    "| 1 | 2 |",
    "",
    "- [ ] open",
    "- [x] done",
    "",
    "A note[^1].",
    "",
    "```go",
    "func main() {}",
    "```",
    "",
    "[^1]: The note.",
    "",
  ].join("\n");
  const created = await createPage(api, pat, notebook.id, "Plan", null, content);
  const page = await signedInPage(tokens);
  const worker = page.waitForEvent("worker");

  await page.goto(wikiPagePath(workspace.slug, notebook.id, created.id));

  await expect(pageHeading(page, "Plan")).toBeVisible();
  const article = page.getByRole("article");
  await expect(article.locator(":scope > :first-child")).toHaveClass("nw-props");
  await expect(article.locator("table.nw-props")).toContainText("statusdraft");
  await expect(article.getByRole("heading", { level: 1, name: "Q4", exact: true })).toBeVisible();
  await expect(article.locator('th[align="left"]')).toHaveText("a");
  await expect(article.getByRole("checkbox")).toHaveCount(2);
  await expect(article.getByRole("checkbox").nth(1)).toBeChecked();
  await expect(article.locator(".footnotes")).toContainText("The note.");

  const keyword = article.locator("pre > code.language-go span.hljs-keyword").first();
  await expect(keyword).toHaveText("func");
  const colours = await keyword.evaluate((span) => [
    getComputedStyle(span).color,
    getComputedStyle(span.closest("code") ?? span).color,
  ]);
  expect(colours[0]).not.toBe(colours[1]);
  const script = new URL((await worker).url());
  expect(script.origin).toBe(new URL(page.url()).origin);
  expect(script.pathname).toMatch(/^\/assets\/highlight\.worker-[\w-]+\.js$/);
  // The worker runs under its own answer's policy: it loads and sends nothing.
  expect((await page.request.get(script.href)).headers()["content-security-policy"]).toBe("default-src 'none'");
});
