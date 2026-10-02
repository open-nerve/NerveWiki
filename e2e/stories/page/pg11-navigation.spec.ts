import { createNotebook } from "../../fixtures/notebooks";
import { createPage, getPage, listNodes, postPage } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { newTeam } from "../../fixtures/workspaces";

// PG11, navigation (M4 design 3): the API's part, the ancestors and the
// tree's order; the page version comes with the tree (M4/P5).

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
