import { pngBytes, uploadAsset } from "../../fixtures/assets";
import { expectIndexedLinks } from "../../fixtures/assert/links";
import {
  expectChangesetOf,
  expectRelocated,
  expectSubtreeDeleted,
  expectTrashRoot,
  placeOf,
} from "../../fixtures/assert/page";
import { errorOf, mcpClient, resultOf } from "../../fixtures/mcp";
import { createNotebook } from "../../fixtures/notebooks";
import { createPage, readContent } from "../../fixtures/pages";
import { expect, test } from "../../fixtures/test";
import { trashListed, trashPath } from "../../fixtures/trash";
import { newOnboardedTeam } from "../../fixtures/workspaces";

// MC4, an agent moves, renames and deletes over MCP (M9 design 9; M9/P2
// design 4.3): a page moved and renamed in one step, the links to it
// following, an attachment renamed, its embed following; the refusals of
// a name without its extension, of another kind, of a name taken and of a
// page with children; a move that leaves the links as they are; and a
// deletion with what is under it, which the web's trash lists.

/** A move's answer, as the tool's structuredContent has it. */
interface Moved {
  path: string;
  node_id: string;
  changed: boolean;
  changeset_id: string | null;
  at: string | null;
}

/** A deletion's answer. */
interface Deleted {
  node_id: string;
  pages: number;
  attachments: number;
}

/** Index's content: two links to Setup and an embed of the attachment under it. */
const index = "[[Setup]] and [[Setup|the setup]] ![[diagram.png]]\n";

test("MC4 (MCP): an agent moves and renames Setup in one step, Index's links following, and renames its attachment, the embed following; a rename without an extension or to a page is refused; a move without its links leaves Index's bytes, its links resolving nowhere; a name taken and a page with children are refused; a recursive deletion puts the subtree in the trash", async ({
  api,
  db,
  nervewiki,
  signedInPage,
}, testInfo) => {
  const { adminId, pat, tokens, workspace } = await newOnboardedTeam(api, testInfo);
  const notebook = await createNotebook(api, pat, workspace.slug, "Handbook");
  const guides = await createPage(api, pat, notebook.id, "Guides");
  const setup = await createPage(api, pat, notebook.id, "Setup");
  const indexPage = await createPage(api, pat, notebook.id, "Index", null, index);
  const diagram = await uploadAsset(api, pat, notebook.id, { name: "diagram.png", bytes: pngBytes }, setup.id);
  const agent = await mcpClient(nervewiki.baseURL, pat, { notebook: notebook.id });

  const setupWas = { ...(await placeOf(db, setup.id)), name: "Setup" };
  const install = resultOf<Moved>(
    await agent.call("move", { from: "Setup.md", to: "Guides/Install.md" }),
    "move Setup"
  );
  expect([install.path, install.node_id, install.changed]).toEqual(["Guides/Install.md", setup.id, true]);
  await expectRelocated(db, setup.id, setupWas, { parentId: guides.id, name: "Install" }, adminId, String(install.at));
  expect((await readContent(api, pat, indexPage.id)).content).toBe(
    "[[Install]] and [[Install|the setup]] ![[diagram.png]]\n"
  );
  await expectIndexedLinks(db, indexPage.id, [
    { kind: "wikilink", property: null, target: "Install", resolved: setup.id },
    { kind: "wikilink", property: null, target: "Install", resolved: setup.id },
    { kind: "embed", property: null, target: "diagram.png", resolved: diagram.id },
  ]);
  await expectChangesetOf(db, String(install.changeset_id), {
    client: "mcp:e2e-agent",
    byId: adminId,
    items: [setup.id],
    versions: [indexPage.id],
  });

  const diagramWas = { ...(await placeOf(db, diagram.id)), name: "diagram.png" };
  const flow = resultOf<Moved>(
    await agent.call("move", { from: "Guides/Install/diagram.png", to: "Guides/Install/flow.png" }),
    "rename the attachment"
  );
  expect(flow.path).toBe("Guides/Install/flow.png");
  await expectRelocated(db, diagram.id, diagramWas, { parentId: setup.id, name: "flow.png" }, adminId, String(flow.at));
  expect((await readContent(api, pat, indexPage.id)).content).toBe(
    "[[Install]] and [[Install|the setup]] ![[flow.png]]\n"
  );
  // A name without an extension, and a page's path: each refused on to, writing nothing.
  const renames = ["Guides/Install/flow", "Guides/Install/flow.md"];
  const refusals = await Promise.all(renames.map((to) => agent.call("move", { from: "Guides/Install/flow.png", to })));
  expect(
    refusals.map((answer, i) => {
      const refused = errorOf(answer, `move the attachment to ${renames[i]}`);
      return [refused.code, (refused.fields as { field: string }[]).map((f) => f.field)];
    })
  ).toEqual([
    ["validation_failed", ["to"]],
    ["validation_failed", ["to"]],
  ]);

  const before = await readContent(api, pat, indexPage.id);
  resultOf<Moved>(
    await agent.call("move", { from: "Guides/Install.md", to: "Guides/Setup guide.md", update_links: false }),
    "move without the links"
  );
  expect(await readContent(api, pat, indexPage.id)).toEqual(before);
  await expectIndexedLinks(db, indexPage.id, [
    { kind: "wikilink", property: null, target: "Install", resolved: null },
    { kind: "wikilink", property: null, target: "Install", resolved: null },
    { kind: "embed", property: null, target: "flow.png", resolved: diagram.id },
  ]);

  const taken = errorOf(
    await agent.call("move", { from: "Guides/Setup guide.md", to: "Guides.md" }),
    "move onto Guides"
  );
  expect(taken.code).toBe("page.title_taken");
  expect(taken.message).toContain("to is the full new path");
  const children = errorOf(await agent.call("delete", { path: "Guides.md" }), "delete Guides");
  expect(children.code).toBe("validation_failed");
  expect(children.message).toContain("1 page and 1 attachment");
  const gone = resultOf<Deleted>(
    await agent.call("delete", { path: "Guides.md", recursive: true }),
    "delete Guides with recursive"
  );
  expect([gone.node_id, gone.pages, gone.attachments]).toEqual([guides.id, 2, 1]);
  await expectSubtreeDeleted(db, guides.id, [guides.id, setup.id], adminId);
  await expectTrashRoot(db, guides.id, { pages: 2, assets: 1, height: 2 });

  const page = await signedInPage(tokens);
  await page.goto(trashPath(workspace.slug, notebook.id));
  await expect(trashListed(page)).toHaveCount(1);
  await expect(trashListed(page).first()).toContainText("Guides");
});
