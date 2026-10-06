import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// L4, tags, formulas and diagrams (M6 design 9; M6/P6 design 8, 10): a
// tag leads to its pages, those of a tag under it too; a formula is
// typeset by KaTeX and a diagram drawn by mermaid, each loaded with the
// first one, under the pages' Content-Security-Policy, which the page's
// watch checks as the test ends: no violation, no console error.

test("L4 (page): a tag leads to the pages of the tag and those under it; formulas and a diagram are typeset and drawn under the CSP", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const content = [
    "Tagged #proj/alpha.",
    "",
    "Inline $E = mc^2$, and a block:",
    "",
    "$$",
    String.raw`\int_0^1 x\,dx = \frac{1}{2}`,
    "$$",
    "",
    "```mermaid",
    "graph TD; Start-->Finish",
    "```",
    "",
  ].join("\n");
  const source = await createPage(api, pat, notebook.id, "Source", null, content);
  const deeper = await createPage(api, pat, notebook.id, "Deeper", null, "#proj/alpha/beta\n");
  await createPage(api, pat, notebook.id, "Other", null, "#proj\n");
  const page = await signedInPage(tokens);
  const katex = page.waitForResponse((response) => /\/assets\/katex-[^/]*\.js$/.test(response.url()));
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  const article = page.getByRole("article");

  // The formulas, typeset: KaTeX's markup, and its MathML for a screen reader.
  await katex;
  await expect(article.locator(".nw-math .katex")).toHaveCount(2);
  await expect(article.locator(".nw-math-block .katex-display math")).toHaveCount(1);
  // The diagram, drawn in its block's place.
  const diagram = article.locator(".nw-diagram svg");
  await expect(diagram).toBeVisible();
  await expect(diagram).toContainText("Start");
  await expect(article.locator("code.language-mermaid")).toHaveCount(0);

  // The tag leads to its pages, the nested tag's among them, not those of the tag above it.
  await article.getByRole("link", { name: "#proj/alpha", exact: true }).click();
  await expect(page.getByRole("heading", { level: 1, name: "#proj/alpha", exact: true })).toBeFocused();
  await expect(page).toHaveURL(`/${workspace.slug}/notebooks/${notebook.id}/tags/proj%2Falpha`);
  const pages = page.getByRole("list", { name: "Pages tagged #proj/alpha", exact: true });
  await expect(pages.getByRole("link")).toHaveText(["Source", "Deeper"]);
  await pages.getByRole("link", { name: "Deeper", exact: true }).click();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, deeper.id));
});
