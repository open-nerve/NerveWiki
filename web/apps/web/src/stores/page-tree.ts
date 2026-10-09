import { titleKey } from "../lib/title-key";
import type { TreeNode } from "../services/page.service";

/**
 * TreeIndex is a notebook's tree looked up: each page by its id, the pages
 * under each parent (null: the root), and how many pages have each title,
 * by its key.
 */
export type TreeIndex = {
  byId: ReadonlyMap<string, TreeNode>;
  children: ReadonlyMap<string | null, readonly TreeNode[]>;
  titles: ReadonlyMap<string, number>;
};

/**
 * indexTree indexes nodes, the tree as the server lists it: parents before
 * their children, siblings in their order, which the children keep.
 */
export function indexTree(nodes: readonly TreeNode[]): TreeIndex {
  const byId = new Map<string, TreeNode>();
  const children = new Map<string | null, TreeNode[]>();
  const titles = new Map<string, number>();
  for (const node of nodes) {
    byId.set(node.id, node);
    const key = titleKey(node.name);
    titles.set(key, (titles.get(key) ?? 0) + 1);
    const siblings = children.get(node.parent_id);
    if (siblings === undefined) {
      children.set(node.parent_id, [node]);
    } else {
      siblings.push(node);
    }
  }
  return { byId, children, titles };
}

/** childrenOf are the pages right under parent (null: the root), in their order. */
export function childrenOf(tree: TreeIndex, parent: string | null): readonly TreeNode[] {
  return tree.children.get(parent) ?? [];
}

/**
 * inTreeOrder are the pages of ids that the tree has, in the order the
 * left column shows them: each before the pages under it, siblings in
 * their order.
 */
export function inTreeOrder(tree: TreeIndex, ids: ReadonlySet<string>): TreeNode[] {
  const found: TreeNode[] = [];
  const visit = (parent: string | null) => {
    for (const node of childrenOf(tree, parent)) {
      if (ids.has(node.id)) {
        found.push(node);
      }
      visit(node.id);
    }
  };
  visit(null);
  return found;
}

/**
 * ancestorsOf are the pages above the page id, from the root down to its
 * parent: none at the root, or for a page not in the tree.
 */
export function ancestorsOf(tree: TreeIndex, id: string): TreeNode[] {
  const ancestors: TreeNode[] = [];
  let parent = tree.byId.get(id)?.parent_id ?? null;
  // A tree has no cycle; the bound keeps a broken one from looping.
  while (parent !== null && ancestors.length < tree.byId.size) {
    const node = tree.byId.get(parent);
    if (node === undefined) {
      break;
    }
    ancestors.unshift(node);
    parent = node.parent_id;
  }
  return ancestors;
}

/**
 * placeOfTitle tells the page id from the other pages of the notebook with
 * its title, compared as titles are (v0.1 design 13.2, item 17): the
 * titles of the pages it is under, from the root down, joined by " / ", or
 * "" at the root; undefined when no other page has its title.
 */
export function placeOfTitle(tree: TreeIndex, id: string): string | undefined {
  const node = tree.byId.get(id);
  if (node === undefined || (tree.titles.get(titleKey(node.name)) ?? 0) < 2) {
    return undefined;
  }
  return ancestorsOf(tree, id)
    .map((page) => page.name)
    .join(" / ");
}

/**
 * maxDepth is how deep pages nest: a root page is at depth 1, and none
 * deeper than the server's domain.MaxDepth (M4 design 4), which the server
 * checks again.
 */
export const maxDepth = 10;

/** depthOf is the depth of the page id: 1 at the root; 0 for none, the root itself. */
export function depthOf(tree: TreeIndex, id: string | null): number {
  return id === null ? 0 : ancestorsOf(tree, id).length + 1;
}

/** subtreeOf is the page id and every page under it, the page first. */
export function subtreeOf(tree: TreeIndex, id: string): TreeNode[] {
  const top = tree.byId.get(id);
  if (top === undefined) {
    return [];
  }
  const pages = [top];
  for (let i = 0; i < pages.length && pages.length <= tree.byId.size; i++) {
    pages.push(...childrenOf(tree, pages[i]?.id ?? null));
  }
  return pages;
}

/** heightOf is how many levels the subtree of the page id has: 1 for a page without children. */
export function heightOf(tree: TreeIndex, id: string): number {
  const top = depthOf(tree, id);
  return Math.max(0, ...subtreeOf(tree, id).map((page) => depthOf(tree, page.id) - top + 1));
}

/**
 * canHold tells whether parent (null: the root) can take the page id with
 * its subtree: not the page itself nor a page under it, and no page deeper
 * than maxDepth (M4/P5 design 3.7). A move the tree forbids is not sent.
 */
export function canHold(tree: TreeIndex, parent: string | null, id: string): boolean {
  if (parent !== null && subtreeOf(tree, id).some((page) => page.id === parent)) {
    return false;
  }
  return depthOf(tree, parent) + heightOf(tree, id) <= maxDepth;
}

/**
 * freeTitle is the first of titles (Untitled, Untitled 2, …), counting from
 * 1, that no sibling, a page or an attachment (M7/P2 design 3.10), and
 * none of taken has (M4 design 4).
 */
export function freeTitle(
  siblings: readonly TreeNode[],
  taken: readonly string[],
  title: (n: number) => string
): string {
  const used = new Set([...siblings.map((page) => titleKey(page.name)), ...taken.map(titleKey)]);
  for (let n = 1; ; n++) {
    if (!used.has(titleKey(title(n)))) {
      return title(n);
    }
  }
}

/**
 * findPages are the pages of tree whose title holds query, compared as
 * titles are (NFC, lower case), from the top down as the left column
 * lists them; every page for a query of blanks (M4/P5 design 3.10).
 */
export function findPages(tree: TreeIndex, query: string): TreeNode[] {
  const key = titleKey(query.trim());
  const found: TreeNode[] = [];
  const visit = (parent: string | null) => {
    for (const page of childrenOf(tree, parent)) {
      if (titleKey(page.name).includes(key)) {
        found.push(page);
      }
      visit(page.id);
    }
  };
  visit(null);
  return found;
}
