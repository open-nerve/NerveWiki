import { expect, test } from "vitest";

import type { PageView, TreeNode } from "../services/page.service";
import { guide, install, linux, notes } from "../test/page-server";
import { PageTreeStore } from "./page-tree.store";

function store(nodes: TreeNode[] = [guide, install, linux, notes]) {
  const read: string[] = [];
  const service = {
    listNodes: async (notebookId: string) => {
      read.push(notebookId);
      return nodes;
    },
    getPageView: async (id: string): Promise<PageView> => ({ html: `<p>${id}</p>`, revision: 1 }),
  };
  return { pages: new PageTreeStore(service, "plans"), read };
}

test("the tree is read for its notebook; before, it has no page", async () => {
  const { pages, read } = store();
  expect(pages.byId(guide.id)).toBeUndefined();
  expect(pages.childrenOf(null)).toEqual([]);

  await pages.load();

  expect(read).toEqual(["plans"]);
  expect(pages.byId(install.id)).toBe(install);
  expect(pages.childrenOf(null)).toEqual([guide, notes]);
  expect(pages.ancestorsOf(linux.id)).toEqual([guide, install]);
});

test("a page is opened and closed; opening to a page opens its ancestors", async () => {
  const { pages } = store();
  await pages.load();

  pages.toggle(notes.id);
  expect(pages.isOpen(notes.id)).toBe(true);
  pages.toggle(notes.id);
  expect(pages.isOpen(notes.id)).toBe(false);

  pages.openTo(linux.id);
  expect([guide, install, linux].map((n) => pages.isOpen(n.id))).toEqual([true, true, false]);
});
