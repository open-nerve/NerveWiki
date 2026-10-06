import { createNotebook } from "../../fixtures/notebooks";
import { createPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// L4, tags, formulas and diagrams (M6 design 9; M6/P6 design 8, 10): a
// tag leads to its pages, those of a tag under it too; a formula is
// typeset by KaTeX and a diagram drawn by mermaid, each loaded with the
// first one, under the pages' Content-Security-Policy, which the page's
// watch checks as the test ends: no violation, no console error. The
// diagram follows the theme.

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
  // KaTeX's stylesheet and fonts: its MathML out of sight, the view scrolling not down.
  await expect(article.locator(".katex").first()).toHaveCSS("font-family", /KaTeX_Main/);
  await expect(article.locator(".katex-mathml").first()).toHaveCSS("position", "absolute");
  expect(await article.evaluate((view) => view.scrollHeight <= view.clientHeight)).toBe(true);
  // The diagram, drawn in its block's place.
  const diagram = article.locator(".nw-diagram svg");
  await expect(diagram).toBeVisible();
  await expect(diagram).toContainText("Start");
  await expect(article.locator("code.language-mermaid")).toHaveCount(0);

  // The system's theme changes as one reads: the diagram is drawn again in it, in its wrapper; the view is not run
  // again, and the focus stays on the tag.
  const tag = article.getByRole("link", { name: "#proj/alpha", exact: true });
  await tag.focus();
  const wrapper = await article.locator(".nw-diagram").elementHandle();
  const light = (await diagram.getAttribute("id")) ?? "";
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(diagram).not.toHaveAttribute("id", light);
  await expect(diagram).toContainText("Start");
  expect(await wrapper?.evaluate((element) => element.isConnected)).toBe(true);
  await expect(tag).toBeFocused();

  // The tag leads to its pages, the nested tag's among them, not those of the tag above it.
  await tag.click();
  await expect(page.getByRole("heading", { level: 1, name: "#proj/alpha", exact: true })).toBeFocused();
  await expect(page).toHaveURL(`/${workspace.slug}/notebooks/${notebook.id}/tags/proj%2Falpha`);
  const pages = page.getByRole("list", { name: "Pages tagged #proj/alpha", exact: true });
  await expect(pages.getByRole("link")).toHaveText(["Source", "Deeper"]);
  await pages.getByRole("link", { name: "Deeper", exact: true }).click();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, deeper.id));
});

test("L4 (page): what a writer's formulas and diagrams could do to a reader's page they cannot: wide, painted outside, expanding without end, or marked up", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const cells = Array.from({ length: 40 }, (_, i) => `<td>column-${i.toString().padStart(2, "0")}</td>`).join("");
  const a = String.raw`\def\a{xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx}`;
  const bomb = a + String.raw`\def\b{${String.raw`\a`.repeat(31)}}` + String.raw`\b`.repeat(31);
  const content = [
    "```mermaid",
    "graph LR; Twin-->Other",
    "```",
    "",
    "```mermaid",
    "graph LR; Twin-->Other",
    "```",
    "",
    "```mermaid",
    '%%{init: {"securityLevel": "loose", "dompurifyConfig": {"ADD_ATTR": ["onmouseover"]}}}%%',
    "graph TD",
    `  A["<b onmouseover=alert(1)>bold</b><a href='javascript:alert(2)'>go</a><a id='nw-forged' class='nw-unresolved' data-nw-target='Forged'>label</a><style>body{display:none}</style>"] --> B`,
    '  click A href "javascript:alert(3)"',
    "```",
    "",
    "```mermaid",
    String.raw`graph TD; M["$$\def\a{x}\a$$"]`,
    "```",
    "",
    `<table><tr>${cells}</tr></table>`,
    "",
    String.raw`A long one $\left(${"x".repeat(300)}\right)$ in a line.`,
    "",
    String.raw`Painted aside $\kern-40em\raisebox{1em}{\colorbox{red}{\rule{0pt}{3em}\text{FAKE BANNER}}}$ here.`,
    "",
    `Expanding $${bomb}$ without end.`,
    "",
  ].join("\n");
  const wide = await createPage(api, pat, notebook.id, "Wide", null, content);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, wide.id));
  const article = page.getByRole("article");
  await expect(article.locator(".nw-math .katex")).toHaveCount(2);

  // Two diagrams of a source, the second one kept: no id of one is another's.
  await expect(article.locator(".nw-diagram svg")).toHaveCount(3);
  const ids = await article.evaluate((view) =>
    [...view.querySelectorAll(".nw-diagram [id]")].map((element) => element.id)
  );
  expect(ids.length).toBeGreaterThan(3);
  expect(new Set(ids).size).toBe(ids.length);

  // A label's markup: no handler, no javascript: address, no id of its own, nothing the app's enhancements take
  // as theirs; its style element none; the directive changes nothing.
  const hostile = article.locator(".nw-diagram").nth(2);
  await expect(hostile).toContainText("label");
  const marked = await hostile.evaluate((diagram) => ({
    handlers: [...diagram.querySelectorAll("*")].flatMap((element) =>
      [...element.attributes].filter((attribute) => attribute.name.startsWith("on")).map((each) => each.name)
    ),
    scripts: [...diagram.querySelectorAll("a")].filter((link) =>
      /^\s*javascript:/i.test(link.getAttribute("href") ?? link.getAttribute("xlink:href") ?? "")
    ).length,
    forged: diagram.querySelectorAll("#nw-forged, [role=button], foreignObject style").length,
  }));
  expect(marked).toEqual({ handlers: [], scripts: 0, forged: 0 });
  await expect(page.locator("body")).toBeVisible();

  // A label's formula that defines a macro: the diagram shows its source.
  await expect(article.locator("code.language-mermaid")).toHaveCount(1);
  await expect(article.locator("code.language-mermaid")).toContainText(String.raw`\def\a`);

  // A formula that would expand without end shows its TeX.
  await expect(article.locator(".nw-math").last()).toHaveText(bomb);
  await expect(article.locator(".nw-math").last().locator(".katex")).toHaveCount(0);

  // What is wide without a region of its own scrolls in the view, which takes the focus, not the page.
  await expect(article).toHaveAttribute("tabindex", "0");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(
    true
  );

  // What a formula paints past the view is not seen, nor clicked: the app beside it is.
  await article.locator("p", { hasText: "Painted aside" }).scrollIntoViewIfNeeded();
  const aside = await article.evaluate((view) => {
    const box = view.querySelector('.nw-math [style*="background-color"]')?.getBoundingClientRect();
    if (box === undefined) {
      return "no box";
    }
    const x = Math.max(box.left, 0) + 2;
    const y = box.top + box.height / 2;
    if (x >= view.getBoundingClientRect().left - 8 || y < 0 || y > window.innerHeight) {
      return `not aside: ${JSON.stringify(box)}`;
    }
    return document.elementFromPoint(x, y)?.closest(".nw-math") ? "paints over the app" : "kept in the view";
  });
  expect(aside).toBe("kept in the view");
});
