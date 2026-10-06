import { expectIndexedLinks } from "../../fixtures/assert/links";
import { countAnswers } from "../../fixtures/browser";
import { holdStream } from "../../fixtures/events";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getView, readContent, renameNode } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { pageHeading, wikiPagePath } from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// L1, a link to a page (M6 design 9; M6/P3 design 6.2, 6.7; M6/P4 design
// 8): a wikilink and a Markdown link to a page of the notebook lead there,
// and to its anchor's heading; one to a page not there shows so, with no
// address, until the page is created elsewhere (the links event); a link
// leads to its page still once the page is renamed elsewhere, written
// again.

test("L1 (API): a reading view's links to pages carry the page each leads to and its anchor's heading, or that it leads nowhere, and no address; one leads to its page once it is created, and once it is renamed, written again", async ({
  api,
  db,
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
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Target", resolved: target.id },
    { kind: "wikilink", property: null, target: "Missing", resolved: null },
    { kind: "link", property: null, target: "Target.md", resolved: target.id },
  ]);

  // The page created, the link leads to it.
  const missing = await createPage(api, pat, notebook.id, "Missing");
  expect((await getView(api, pat, source.id)).data?.html).toContain(
    `<a class="nw-wikilink" data-nw-node="${missing.id}">Missing</a>`
  );
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Target", resolved: target.id },
    { kind: "wikilink", property: null, target: "Missing", resolved: missing.id },
    { kind: "link", property: null, target: "Target.md", resolved: target.id },
  ]);

  // The page renamed, the links to it are written again, and lead to it.
  expect((await renameNode(api, pat, target.id, "Renamed")).response.status).toBe(200);
  expect((await readContent(api, pat, source.id)).content).toBe(
    "[[Renamed#Part Two|see]] [[Missing]] [md](Renamed.md)\n"
  );
  expect((await getView(api, pat, source.id)).data?.html).toContain(
    `<a class="nw-wikilink" data-nw-node="${target.id}" data-nw-anchor="nw-part-two">see</a>`
  );
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Renamed", resolved: target.id },
    { kind: "wikilink", property: null, target: "Missing", resolved: missing.id },
    { kind: "link", property: null, target: "Renamed.md", resolved: target.id },
  ]);
});

test("L1 (page): a link to a page opens it in the app, or in a new tab with a modifier, and goes to its anchor's heading; one to a page not there is no link", async ({
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
  expect(await notReloaded()).toBe(true);
});

test("L1 (page): a link to a page not there leads to it once the page is created elsewhere: the view is read again with the links event, the focus and the scroll kept", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const source = await createPage(
    api,
    pat,
    notebook.id,
    "Source",
    null,
    "[[Missing]]\n\n" + "filler\n\n".repeat(80) + "# Part A\n\n[[#Gone]] [[#Part A|to part a]]\n"
  );
  const page = await signedInPage(tokens);
  const letStreamIn = await holdStream(page);
  const viewReads = countAnswers(page, "GET", `/api/v0/pages/${source.id}/view`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  const article = page.getByRole("article", { name: "Source" });
  await expect(article.getByText("Missing", { exact: true })).toHaveClass(/nw-unresolved/);
  // The stream connects, and its refresh reads the view again: from then on, the page created comes as events.
  letStreamIn();
  await expect.poll(viewReads).toBeGreaterThanOrEqual(2);

  // A link of the page to no heading leaves the focus where the browser put it, and the page where it is, once the
  // view has taken the address in (a frame); one to a heading goes there.
  const gone = article.getByRole("link", { name: "Gone", exact: true });
  await gone.click();
  await expect(page).toHaveURL(/#nw-gone$/);
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => setTimeout(resolve, 0))));
  await expect(pageHeading(page, "Source")).not.toBeFocused();
  await expect(gone).toBeInViewport();
  await article.getByRole("link", { name: "to part a", exact: true }).click();
  const partA = article.getByRole("heading", { name: "Part A", exact: true });
  await expect(partA).toBeFocused();
  await expect(partA).toBeInViewport();
  const read = viewReads();

  const missing = await createPage(api, pat, notebook.id, "Missing");
  await expect(article.getByRole("link", { name: "Missing", exact: true })).toHaveAttribute(
    "href",
    wikiPagePath(workspace.slug, notebook.id, missing.id)
  );
  expect(viewReads()).toBeGreaterThan(read);
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Missing", resolved: missing.id },
  ]);
  // Read again, the view keeps the heading's focus and stays where it was.
  await expect(partA).toBeFocused();
  await expect(partA).toBeInViewport();
});

test("L1 (page): a link leads to its page once the page is renamed elsewhere: the rename writes the link again, and the view is read again with the event", async ({
  api,
  db,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const target = await createPage(api, pat, notebook.id, "Target");
  const source = await createPage(api, pat, notebook.id, "Source", null, "[[Target]]\n");
  const targetPath = wikiPagePath(workspace.slug, notebook.id, target.id);
  const page = await signedInPage(tokens);
  const letStreamIn = await holdStream(page);
  const viewReads = countAnswers(page, "GET", `/api/v0/pages/${source.id}/view`);
  await page.goto(wikiPagePath(workspace.slug, notebook.id, source.id));
  const article = page.getByRole("article", { name: "Source" });
  await expect(article.getByRole("link", { name: "Target", exact: true })).toHaveAttribute("href", targetPath);
  // The stream connects, and its refresh reads the view again: from then on, the rename comes as events.
  letStreamIn();
  await expect.poll(viewReads).toBeGreaterThanOrEqual(2);

  expect((await renameNode(api, pat, target.id, "Renamed")).response.status).toBe(200);
  const renamed = article.getByRole("link", { name: "Renamed", exact: true });
  await expect(renamed).toHaveAttribute("href", targetPath);
  await expectIndexedLinks(db, source.id, [
    { kind: "wikilink", property: null, target: "Renamed", resolved: target.id },
  ]);
  await renamed.click();
  await expect(pageHeading(page, "Renamed")).toBeFocused();
});
