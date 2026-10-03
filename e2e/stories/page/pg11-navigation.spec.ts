import { notebookPath } from "../../fixtures/notebook-pages";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getPage, listNodes, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import {
  breadcrumbs,
  pageHeading,
  quickSwitchFor,
  subpages,
  treeTitles,
  wikiPagePath,
} from "../../fixtures/wiki-pages";
import { newOnboardedTeam, newTeam } from "../../fixtures/workspaces";

// PG11, navigation (M4 design 3): the API's part, the ancestors and the
// tree's order; in the browser (M4/P5 design 3.5, 3.10), the breadcrumbs,
// the subpages and the quick switch.

test("PG11 (API): a page reads its ancestors from the root; the tree lists each parent before its children, siblings in their order", async ({
  api,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const a = await createPage(api, pat, notebook.id, "A");
  const b = await createPage(api, pat, notebook.id, "B", a.id);
  const c = await createPage(api, pat, notebook.id, "C", b.id);
  await createPage(api, pat, notebook.id, "Last");
  const first = await postPage(api, pat, notebook.id, { parent_id: a.id, title: "First", after_id: null });
  expect(first.response.status).toBe(201);
  const between = await postPage(api, pat, notebook.id, {
    parent_id: a.id,
    title: "Between",
    after_id: first.data?.id ?? null,
  });
  expect(between.response.status).toBe(201);

  const read = await getPage(api, pat, c.id);
  expect(read.response.status).toBe(200);
  expect(read.data?.ancestors).toEqual([
    { id: a.id, name: "A" },
    { id: b.id, name: "B" },
  ]);
  const tree = await listNodes(api, pat, notebook.id);
  expect(tree.map((n) => n.name)).toEqual(["A", "First", "Between", "B", "C", "Last"]);
  expect(tree.find((n) => n.id === c.id)?.parent_id).toBe(b.id);
});

test("PG11 (page): a page shows where it is in its breadcrumbs, which go up, and lists its subpages; the home lists the root pages; Ctrl+O finds a page by its title and goes to it", async ({
  api,
  signedInPage,
}, testInfo) => {
  const { pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const guide = await createPage(api, pat, notebook.id, "Guide");
  const install = await createPage(api, pat, notebook.id, "Install", guide.id);
  const linux = await createPage(api, pat, notebook.id, "Linux", install.id);
  await createPage(api, pat, notebook.id, "Notes");
  const page = await signedInPage(tokens);

  await page.goto(wikiPagePath(workspace.slug, notebook.id, install.id));
  await expect(pageHeading(page, "Install")).toBeVisible();
  expect(await breadcrumbs(page)).toEqual(["Plans", "Guide", "Install"]);
  expect(await subpages(page)).toEqual(["Linux"]);
  // The tree shows the page: its ancestors open.
  expect(await treeTitles(page, "Plans")).toEqual(["Guide", "Install", "Notes"]);

  await page.getByRole("navigation", { name: "Breadcrumb", exact: true }).getByRole("link", { name: "Plans" }).click();
  await expect(page).toHaveURL(notebookPath(workspace.slug, notebook.id));
  expect(
    await page.getByRole("main").getByRole("list", { name: "Pages", exact: true }).getByRole("link").allTextContents()
  ).toEqual(["Guide", "Notes"]);

  expect(await quickSwitchFor(page, "LIN")).toEqual([["Linux", "Guide / Install"]]);
  await page.keyboard.press("Enter");
  await expect(pageHeading(page, "Linux")).toBeFocused();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, linux.id));
  expect(await breadcrumbs(page)).toEqual(["Plans", "Guide", "Install", "Linux"]);
});
