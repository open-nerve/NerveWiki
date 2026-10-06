import type { Page } from "@playwright/test";

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

/** mindmap is a mindmap's source: a root and its children, a line each. */
function mindmap(children: number): string {
  return ["mindmap", "  root", ...Array.from({ length: children }, (_, i) => `    node${i}`)].join("\n");
}

/** separated is a mindmap's source whose children are on one line: each after a line separator, ended by a comment. */
function separated(children: number): string {
  return ["mindmap\n  root", ...Array.from({ length: children }, (_, i) => `\u2028    l${i}[leaf${i}]%%`)].join("");
}

/** frames lets the page draw twice: what came into sight is seen. */
const frames = (page: Page) =>
  page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));

test("L4 (page): what a writer's formulas and diagrams could do to a reader's page they cannot: wide, painted outside, expanding without end, crashing it, or marked up", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const cells = Array.from({ length: 40 }, (_, i) => `<td>column-${i.toString().padStart(2, "0")}</td>`).join("");
  const a = String.raw`\def\a{xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx}`;
  const bomb = a + String.raw`\def\b{${String.raw`\a`.repeat(31)}}` + String.raw`\b`.repeat(31);
  const columns = String.raw`\begin{alignedat}{30000000}a\end{alignedat}`;
  const deep = `${"x^{".repeat(150)}x${"}".repeat(150)}`;
  const styled = `${String.raw`\pmb{`.repeat(160)}x${"}".repeat(160)}`;
  // The diagrams mermaid cannot draw come first: they are tried, one at a time, before the twins are drawn.
  const content = [
    "Go [[#Far|far]].",
    "",
    "```mermaid",
    String.raw`graph TD; M["$$\def\a{x}\a$$"]`,
    "```",
    "",
    "```mermaid",
    String.raw`graph TD; S["$$\d<x></x>ef\a{x}\a$$"]`,
    "```",
    "",
    "```mermaid",
    `graph TD; C["$$${columns}$$"]`,
    "```",
    "",
    "```mermaid",
    `graph TD; D["$$${deep}$$"]`,
    "```",
    "",
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
    `  A["<b onmouseover=alert(1)>bold</b><a href='javascript:alert(2)'>go</a><a id='nw-forged' class='nw-unresolved' data-nw-target='Forged'>label</a><input type='checkbox' data-task='0'><style>body{display:none}</style>"] --> B`,
    '  click A href "javascript:alert(3)"',
    "```",
    "",
    "| Name | Value |",
    "|---|---|",
    String.raw`| smashed | $\smash[b]{\underbrace{x}_{y}}$ |`,
    "",
    `<table><tr>${cells}</tr></table>`,
    "",
    String.raw`A long one $\left(${"x".repeat(300)}\right)$ in a line.`,
    "",
    String.raw`Painted aside $\kern-40em\raisebox{1em}{\colorbox{red}{\rule{0pt}{3em}\text{FAKE BANNER}}}$ here.`,
    "",
    `Expanding $${bomb}$ without end.`,
    "",
    `Too many columns $${columns}$ here.`,
    "",
    `Too deep $${deep}$ here.`,
    "",
    `Too styled $${styled}$ here.`,
    "",
    "```mermaid",
    mindmap(150),
    "```",
    "",
    "```mermaid",
    separated(160),
    "```",
    "",
    "```mermaid",
    mindmap(30),
    "```",
    "",
    "# Far",
    "",
    ...Array.from({ length: 12 }, (_, i) => `After the heading, ${i}.\n`),
    // Its rule reaches 50em past the view's bottom, which hides it.
    String.raw`The last $\smash[b]{\rule[-50em]{1pt}{1pt}}$ reaches past the view.`,
    "",
  ].join("\n");
  const wide = await createPage(api, pat, notebook.id, "Wide", null, content);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, wide.id));
  const article = page.getByRole("article");
  await expect(article.locator(".nw-math .katex")).toHaveCount(4);

  // Two diagrams of a source, the second one kept: no id of one is another's.
  await expect(article.locator(".nw-diagram svg")).toHaveCount(3);
  const ids = await article.evaluate((view) =>
    [...view.querySelectorAll(".nw-diagram [id]")].map((element) => element.id)
  );
  expect(ids.length).toBeGreaterThan(3);
  expect(new Set(ids).size).toBe(ids.length);

  // A label's markup: no handler, no javascript: address, no id of its own, none of the server's marks that the
  // app's enhancements act on (a task's box, a link's target); its style element none; the directive changes
  // nothing.
  const hostile = article.locator(".nw-diagram").nth(2);
  await expect(hostile).toContainText("label");
  const marked = await hostile.evaluate((diagram) => ({
    handlers: [...diagram.querySelectorAll("*")].flatMap((element) =>
      [...element.attributes].filter((attribute) => attribute.name.startsWith("on")).map((each) => each.name)
    ),
    scripts: [...diagram.querySelectorAll("a")].filter((link) =>
      /^\s*javascript:/i.test(link.getAttribute("href") ?? link.getAttribute("xlink:href") ?? "")
    ).length,
    forged: diagram.querySelectorAll(
      "#nw-forged, [role=button], [data-task], [data-nw-target], [data-nw-node], [data-nw-tag], foreignObject style"
    ).length,
  }));
  expect(marked).toEqual({ handlers: [], scripts: 0, forged: 0 });
  await expect(page.locator("body")).toBeVisible();

  // A mindmap of more lines than its bound, and one of as many nodes on one line, near the end, are tried as they
  // show, before the small one after them, which is drawn: the heading after it shown, which its drawing leaves.
  await article.locator("code.language-mermaid", { hasText: "node149" }).scrollIntoViewIfNeeded();
  await frames(page);
  await article.locator("code.language-mermaid", { hasText: "leaf159" }).scrollIntoViewIfNeeded();
  await frames(page);
  await article.getByRole("heading", { name: "Far", exact: true }).scrollIntoViewIfNeeded();
  await expect(article.locator(".nw-diagram svg")).toHaveCount(4);
  await expect(article.locator(".nw-diagram").last()).toContainText("node29");

  // A label's formula that defines a macro, one that does once mermaid has sanitized it, one of too many columns,
  // one nested too deep, and the large mindmaps: the diagram shows its source.
  await expect(article.locator("code.language-mermaid")).toHaveText([
    String.raw`graph TD; M["$$\def\a{x}\a$$"]`,
    String.raw`graph TD; S["$$\d<x></x>ef\a{x}\a$$"]`,
    `graph TD; C["$$${columns}$$"]`,
    `graph TD; D["$$${deep}$$"]`,
    mindmap(150),
    separated(160),
  ]);

  // A formula that would expand without end, one of too many columns, one nested too deep, and one whose styled
  // groups would take the layout seconds show their TeX; the page is still there.
  await Promise.all(
    [bomb, columns, deep, styled].map(async (tex) => {
      const formula = article.locator(".nw-math").filter({ hasText: tex });
      await expect(formula).toHaveText(tex);
      await expect(formula.locator(".katex")).toHaveCount(0);
    })
  );

  // A formula past the view's bottom does not make the view scroll down: the wheel over it scrolls the page.
  await page.evaluate(() => window.scrollTo(0, 0));
  await article.hover({ position: { x: 2, y: 2 } });
  await page.mouse.wheel(0, 200);
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
  expect(await article.evaluate((view) => view.scrollTop)).toBe(0);
  // Nor over a table whose last row's formula reaches past its wrapper's bottom.
  const wrapper = article.locator(".nw-scroll", { has: page.locator("table", { hasText: "smashed" }) });
  await wrapper.scrollIntoViewIfNeeded();
  const before = await page.evaluate(() => window.scrollY);
  await wrapper.hover();
  await page.mouse.wheel(0, 100);
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(before);
  expect(await wrapper.evaluate((element) => element.scrollTop)).toBe(0);
  // An anchor's scroll, which a hidden overflow lets through, leaves the view where it was, its top in sight; the
  // window scrolls instead, the heading in sight.
  await article.getByRole("link", { name: "far", exact: true }).click();
  const far = article.getByRole("heading", { name: "Far", exact: true });
  await expect(far).toBeFocused();
  await expect.poll(() => article.evaluate((view) => view.scrollTop)).toBe(0);
  await expect(far).toBeInViewport();

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

test("L4 (page): a view's formulas take at most a while to lay out, which the view laid out again takes at once: past it, the rest show their TeX", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  // Four chains of styled groups, each 144 deep: a formula takes the layout a tenth of a second or more.
  const chain = `${String.raw`\pmb{`.repeat(144)}x${"}".repeat(144)}`;
  const heavy = Array.from({ length: 4 }, () => chain).join(" ");
  const content = [...Array.from({ length: 30 }, (_, i) => `Heavy ${i}: $${heavy}$.\n`), "Last $y$.", ""].join("\n");
  const styled = await createPage(api, pat, notebook.id, "Styled", null, content);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, styled.id));
  const article = page.getByRole("article");
  const formulas = article.locator(".nw-math");

  await expect(formulas.first().locator(".katex")).toHaveCount(1);
  // The typesetting ends with the box the formulas are laid out in gone.
  await expect(article.locator(".nw-math-measure")).toHaveCount(0);
  await expect(formulas.last()).toHaveText("y");
  await expect(formulas.last().locator(".katex")).toHaveCount(0);
});

test("L4 (page): a page opened at a heading shows it once the formulas above it are typeset, taller than their TeX; read again, they stay typeset, and what is below stays put", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const content = [
    ...Array.from({ length: 20 }, (_, i) =>
      ["$$", String.raw`\begin{pmatrix}a_{${i}}&b\\c&d\end{pmatrix}`, "$$", ""].join("\n")
    ),
    "# Far",
    "",
    ...Array.from({ length: 6 }, (_, i) => `After the heading, ${i}.\n`),
    "- [ ] Tick me",
    "",
    ...Array.from({ length: 40 }, (_, i) => `After the task, ${i}.\n`),
  ].join("\n");
  const matrices = await createPage(api, pat, notebook.id, "Matrices", null, content);
  const page = await signedInPage(tokens);
  await page.goto(`${wikiPagePath(workspace.slug, notebook.id, matrices.id)}#nw-far`);
  const article = page.getByRole("article");
  const far = article.getByRole("heading", { name: "Far", exact: true });

  await expect(far).toBeFocused();
  await expect(article.locator(".nw-math .katex")).toHaveCount(20);
  await expect(article.locator(".nw-math-measure")).toHaveCount(0);
  await expect(far).toBeInViewport();

  // A task ticked below them, the page is read again: its formulas are put again at once, as they were, and the
  // box, focused, stays where it was.
  const box = article.getByRole("checkbox");
  const top = await box.evaluate((element) => {
    element.dataset.before = "";
    return element.getBoundingClientRect().top;
  });
  await box.click();
  await expect(article.locator("[data-before]")).toHaveCount(0);
  await expect(box).toBeChecked();
  await expect(box).toBeFocused();
  await expect(box).toBeInViewport();
  expect(Math.abs((await box.evaluate((element) => element.getBoundingClientRect().top)) - top)).toBeLessThan(2);
  await expect(article.locator(".nw-math .katex")).toHaveCount(20);
});
