import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// L7, line breaks (M6/P8 design 2): a single line break in a paragraph is
// shown as one, as Obsidian's reading view shows it, in a callout's body
// too; a callout's title ends at its first line. Through the API, the view
// carries the line breaks as <br>; on the page, the lines are shown apart.

/** The page's content: a paragraph of two lines, a callout with a title and a body of two lines. */
const lines = ["第一行", "第二行", "", "> [!note] 标题", "> 正文一", "> 正文二", ""].join("\n");

test("L7 (API): a reading view's paragraph and callout body carry their line breaks, the callout's title none", async ({
  api,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const page = await createPage(api, pat, notebook.id, "Lines", null, lines);

  const html = (await getView(api, pat, page.id)).data?.html ?? "";
  for (const element of [
    "<p>第一行<br>\n第二行</p>",
    '<div class="nw-callout-title">标题</div>',
    "<p>正文一<br>\n正文二</p>",
  ]) {
    expect(html).toContain(element);
  }
});

test("L7 (page): a paragraph's lines and a callout body's are shown apart, the callout's title on its one line", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const source = await createPage(api, pat, notebook.id, "Lines", null, lines);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  const article = page.getByRole("article");

  await expect(article.locator("p").first()).toHaveJSProperty("innerText", "第一行\n第二行");
  const callout = article.locator(".nw-callout");
  await expect(callout.locator(".nw-callout-title")).toHaveJSProperty("innerText", "标题");
  await expect(callout.locator("p")).toHaveJSProperty("innerText", "正文一\n正文二");
  // Each line is a line of its own: the second's text below the first's.
  const [first, second] = await article
    .locator("p")
    .first()
    .evaluate((p) =>
      [...p.childNodes]
        .filter((n) => n.nodeType === Node.TEXT_NODE && n.textContent?.trim())
        .map((n) => {
          const range = document.createRange();
          range.selectNodeContents(n);
          return range.getBoundingClientRect().top;
        })
    );
  expect(second).toBeGreaterThan(first ?? Number.POSITIVE_INFINITY);
});
