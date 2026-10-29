import { autorun, runInAction } from "mobx";
import { expect, onTestFinished, test } from "vitest";

import { workspaceJSON } from "../test/fakes";
import { assetNode, guide, install, linux, notes, pageServer } from "../test/page-server";
import { RootStore } from "./root.store";

test("strict source membership follows all nodes and mode", async () => {
  const attachment = assetNode(21, "Original.pdf", linux);
  const outsideAsset = assetNode(22, "Outside.pdf", notes);
  const server = pageServer({
    mode: "llm_wiki",
    sourceRootId: guide.id,
    nodes: [guide, install, linux, attachment, notes, outsideAsset],
  });
  await server.app.session.start();
  const root = new RootStore(server.app, "login-0");
  const pages = root.pagesOf(server.notebook);
  if (pages === undefined) throw new Error("signed-in fixture has no tree");
  await pages.load();
  const seen: boolean[][] = [];
  onTestFinished(
    autorun(() => seen.push([guide, install, linux, attachment, notes, outsideAsset].map((n) => pages.isSource(n.id))))
  );
  expect(seen.at(-1)).toEqual([false, false, false, false, false, false]);

  // Creating the notebook list after pagesOf must be observable too: no captured initial notebook.
  const notebooks = root.notebooksOf(workspaceJSON);
  if (notebooks === undefined) throw new Error("signed-in fixture has no notebooks");
  await notebooks.load();
  expect(seen.at(-1)).toEqual([false, true, true, true, false, false]);
  expect(pages.nodeById(attachment.id)?.name).toBe("Original.pdf");
  expect(pages.byId(attachment.id)).toBeUndefined();
  expect(pages.nodeAncestors(attachment.id)?.map((n) => n.id)).toEqual([guide.id, install.id, linux.id]);
  expect(pages.nodeAncestors(guide.id)).toEqual([]);
  expect(pages.nodeAncestors("missing")).toBeUndefined();
  expect(pages.isSource("missing")).toBe(false);

  runInAction(() => {
    pages.nodes = [
      { ...guide, name: "Moved sources", parent_id: notes.id },
      install,
      linux,
      attachment,
      notes,
      outsideAsset,
    ];
  });
  expect(seen.at(-1)).toEqual([false, true, true, true, false, false]);
  expect(pages.nodeAncestors(attachment.id)?.map((n) => n.id)).toEqual([notes.id, guide.id, install.id, linux.id]);
  server.notebook = { ...server.notebook, mode: "standard", source_root_id: null };
  await notebooks.load();
  expect(seen.at(-1)).toEqual([false, false, false, false, false, false]);
  server.notebook = { ...server.notebook, mode: "llm_wiki", source_root_id: install.id };
  await notebooks.load();
  expect(seen.at(-1)).toEqual([false, false, true, true, false, false]);
  expect(root.pagesOf(server.notebook)).toBe(pages);
});

test.each(["missing-root", "missing-parent", "cycle", "broken-above-root"] as const)(
  "strict source membership rejects an incomplete ancestry: %s",
  async (shape) => {
    const attachment = assetNode(21, "Original.pdf", linux);
    const nodes = [guide, install, linux, attachment, notes];
    const server = pageServer({ mode: "llm_wiki", sourceRootId: guide.id, nodes });
    await server.app.session.start();
    const root = new RootStore(server.app, "login-0");
    await root.notebooksOf(workspaceJSON)?.load();
    const pages = root.pagesOf(server.notebook);
    if (pages === undefined) throw new Error("signed-in fixture has no tree");
    await pages.load();
    expect(pages.isSource(attachment.id)).toBe(true);
    runInAction(() => {
      pages.nodes =
        shape === "missing-root"
          ? nodes.filter((n) => n.id !== guide.id)
          : shape === "missing-parent"
            ? nodes.filter((n) => n.id !== install.id)
            : shape === "cycle"
              ? nodes.map((n) => (n.id === install.id ? Object.assign({}, n, { parent_id: linux.id }) : n))
              : nodes.map((n) => (n.id === guide.id ? Object.assign({}, n, { parent_id: "missing" }) : n));
    });
    expect(pages.isSource(attachment.id)).toBe(false);
    expect(pages.nodeAncestors(attachment.id)).toBeUndefined();
  }
);
