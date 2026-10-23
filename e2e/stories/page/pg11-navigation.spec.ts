import type { TreeNode } from "@nervewiki/api-client";

import { errorOf, mcpClient, readPage, resultOf, type McpClient } from "../../fixtures/mcp";
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
// the subpages and the quick switch. An agent reads a page by its
// ancestors' path, and lists a folder at a time in the tree's order (M9/P5
// design 3.2).

/** A list's answer, as much as PG11 reads of the tool's structuredContent. */
interface Listing {
  items: { path: string; has_children: boolean }[];
  next_cursor: string | null;
}

/**
 * The paths agent lists under folder ("" for the root, "A/" for A's children) and under each of them that has
 * children, a page before its children's: the tree's order walked folder by folder.
 */
async function walk(agent: McpClient, folder = "", depth = 0): Promise<string[]> {
  expect(depth, `the walk ends above ${folder}`).toBeLessThan(11);
  const listing = resultOf<Listing>(
    await agent.call("list", folder === "" ? {} : { pattern: folder }),
    `list ${folder}`
  );
  expect(listing.next_cursor, `list ${folder} on one page`).toBeNull();
  const paths: string[] = [];
  for (const item of listing.items) {
    paths.push(item.path);
    if (item.has_children) {
      // oxlint-disable-next-line no-await-in-loop -- a page's children before its next sibling
      paths.push(...(await walk(agent, `${item.path.replace(/\.md$/, "")}/`, depth + 1)));
    }
  }
  return paths;
}

/** The path of each of nodes, the tree REST lists, from its ancestors' names joined by parent_id, in its order. */
function pathsOf(nodes: TreeNode[]): string[] {
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const names = (node: TreeNode | undefined): string[] =>
    node === undefined ? [] : [...names(byId.get(node.parent_id ?? "")), node.name];
  return nodes.map((node) => `${names(node).join("/")}${node.kind === "page" ? ".md" : ""}`);
}

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

test("PG11 (MCP): an agent reads a page by its ancestors' path, in any case, and not by another; each folder lists its children in their order, the tree's order walked folder by folder", async ({
  api,
  nervewiki,
}, testInfo) => {
  const { pat, workspace } = await newTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Plans");
  const a = await createPage(api, pat, notebook.id, "A");
  const b = await createPage(api, pat, notebook.id, "B", a.id);
  const c = await createPage(api, pat, notebook.id, "C", b.id);
  await createPage(api, pat, notebook.id, "Last");
  // write takes no place among the siblings: First and Between are placed through REST.
  const first = await postPage(api, pat, notebook.id, { parent_id: a.id, title: "First", after_id: null });
  expect(first.response.status).toBe(201);
  const between = await postPage(api, pat, notebook.id, {
    parent_id: a.id,
    title: "Between",
    after_id: first.data?.id ?? null,
  });
  expect(between.response.status).toBe(201);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  // A path is found by its keys, and answered with the names stored, those of the page's ancestors.
  const read = await readPage(agent, { path: "a/b/c.md" });
  expect([read.meta.path, read.meta.version]).toEqual(["A/B/C.md", `${c.id}:1`]);
  const ancestors = (await getPage(api, pat, c.id)).data?.ancestors ?? [];
  expect(read.meta.path).toBe([...ancestors.map((ancestor) => ancestor.name), "C.md"].join("/"));
  expect(errorOf(await agent.call("read", { path: "B/C.md" }), "read B/C.md").code).toBe("page.not_found");

  // Folder by folder, the tree's order; REST's tree, its paths joined by parent_id, is the same.
  const walked = await walk(agent);
  expect(walked).toEqual(["A.md", "A/First.md", "A/Between.md", "A/B.md", "A/B/C.md", "Last.md"]);
  expect(walked).toEqual(pathsOf(await listNodes(api, pat, notebook.id)));
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
  await expect.poll(() => treeTitles(page, "Plans")).toEqual(["Guide", "Install", "Notes"]);

  await page.getByRole("navigation", { name: "Breadcrumb", exact: true }).getByRole("link", { name: "Plans" }).click();
  await expect(page).toHaveURL(notebookPath(workspace.slug, notebook.id));
  await expect(page.getByRole("main").getByRole("list", { name: "Pages", exact: true }).getByRole("link")).toHaveText([
    "Guide",
    "Notes",
  ]);

  expect(await quickSwitchFor(page, "LIN")).toEqual([["Linux", "Guide / Install"]]);
  await page.keyboard.press("Enter");
  await expect(pageHeading(page, "Linux")).toBeFocused();
  await expect(page).toHaveURL(wikiPagePath(workspace.slug, notebook.id, linux.id));
  expect(await breadcrumbs(page)).toEqual(["Plans", "Guide", "Install", "Linux"]);
});
