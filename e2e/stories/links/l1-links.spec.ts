import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// L1, a link to a page (M6 design 9; M6/P3 design 6.2, 6.7): a wikilink and
// a Markdown link to a page of the notebook lead there, and to its anchor's
// heading; one to a page not there shows so, with no address, until the
// page is created elsewhere (the links event). That a link still leads to
// its page once the page is renamed is P4's half.

test("L1 (API): a reading view's links to pages carry the page each leads to and its anchor's heading, or that it leads nowhere, and no address", async ({
  api,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const target = await createPage(api, pat, notebook.id, "Target", null, "# Part Two\n");
  const source = await createPage(
    api,
    pat,
    notebook.id,
    "Source",
    null,
    "[[Target#Part Two|see]] [[Missing]] [md](Target.md)\n"
  );

  const { data, error, response } = await getView(api, pat, source.id);
  expect(response.status, JSON.stringify(error)).toBe(200);
  const html = data?.html ?? "";
  for (const element of [
    `<a class="nw-wikilink" data-nw-node="${target.id}" data-nw-anchor="nw-part-two">see</a>`,
    '<a class="nw-wikilink nw-unresolved" data-nw-target="Missing">Missing</a>',
    `<a data-nw-node="${target.id}">md</a>`,
  ]) {
    expect(html).toContain(element);
  }
  expect(html).not.toContain("href");
});

test("L1 (page): a link to a page opens it in the app, or in a new tab with a modifier, and goes to its anchor's heading; one to a page not there is no link until the page is created elsewhere", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const target = await createPage(
    api,
    pat,
    notebook.id,
    "Target",
    null,
    "intro\n\n" + "filler\n\n".repeat(80) + "# Part Two\n"
  );
  const source = await createPage(
    api,
    pat,
    notebook.id,
    "Source",
    null,
    "[[Target]] and [[Target#Part Two|part two]] and [[Missing]]\n"
  );
  const targetPath = wikiPagePath(workspace.slug, notebook.id, target.id);
  const page = await signedInPage(tokens);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  const article = page.getByRole("article");
  const toTarget = article.getByRole("link", { name: "Target", exact: true });
  await expect(toTarget).toHaveAttribute("href", targetPath);
  await expect(article.getByRole("link", { name: "part two", exact: true })).toHaveAttribute(
    "href",
    `${targetPath}#nw-part-two`
  );
  await expect(article.getByText("Missing", { exact: true })).toHaveClass(/nw-unresolved/);
  await expect(article.getByRole("link", { name: "Missing", exact: true })).toHaveCount(0);
  // A page that does not load again keeps what was set on its window.
  await page.evaluate(() => Object.assign(window, { notReloaded: true }));
  const notReloaded = () => page.evaluate(() => "notReloaded" in window);

  // With a modifier the browser opens the link's address: in a new tab.
  const [opened] = await Promise.all([
    page.context().waitForEvent("page"),
    toTarget.click({ modifiers: ["ControlOrMeta"] }),
  ]);
  await expect(pageHeading(opened, "Target")).toBeVisible();
  await opened.close();
  await expect(pageHeading(page, "Source")).toBeVisible();

  // A plain click goes through the app's router: the page's heading takes the focus.
  await toTarget.click();
  await expect(pageHeading(page, "Target")).toBeFocused();
  await expect(page).toHaveURL(targetPath);
  expect(await notReloaded()).toBe(true);

  // A link with an anchor goes to its heading, which takes the focus.
  await page.goBack();
  await article.getByRole("link", { name: "part two", exact: true }).click();
  const heading = page.getByRole("article").getByRole("heading", { level: 1, name: "Part Two", exact: true });
  await expect(heading).toBeFocused();
  await expect(heading).toBeInViewport();
  await expect(page).toHaveURL(`${targetPath}#nw-part-two`);

  // The page created elsewhere, the link leads to it: the view is read again with the links event.
  await page.goBack();
  await expect(pageHeading(page, "Source")).toBeVisible();
  const missing = await createPage(api, pat, notebook.id, "Missing");
  await expect(article.getByRole("link", { name: "Missing", exact: true })).toHaveAttribute(
    "href",
    wikiPagePath(workspace.slug, notebook.id, missing.id)
  );
  expect(await notReloaded()).toBe(true);
});
