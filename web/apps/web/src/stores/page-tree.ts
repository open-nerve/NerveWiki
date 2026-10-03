import type { TreeNode } from "../services/page.service";

/** TreeIndex is a notebook's tree looked up: each page by its id, and the pages under each parent (null: the root). */
export type TreeIndex = {
  byId: ReadonlyMap<string, TreeNode>;
  children: ReadonlyMap<string | null, readonly TreeNode[]>;
};

/**
 * indexTree indexes nodes, the tree as the server lists it: parents before
 * their children, siblings in their order, which the children keep.
 */
export function indexTree(nodes: readonly TreeNode[]): TreeIndex {
  const byId = new Map<string, TreeNode>();
  const children = new Map<string | null, TreeNode[]>();
  for (const node of nodes) {
    byId.set(node.id, node);
    const siblings = children.get(node.parent_id);
    if (siblings === undefined) {
      children.set(node.parent_id, [node]);
    } else {
      siblings.push(node);
    }
  }
  return { byId, children };
}

/** childrenOf are the pages right under parent (null: the root), in their order. */
export function childrenOf(tree: TreeIndex, parent: string | null): readonly TreeNode[] {
  return tree.children.get(parent) ?? [];
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
 * titleKey is how titles compare among siblings, near the server's
 * shared.TitleKey: NFC, then lower case. Where the two differ the server
 * answers 409 page.title_taken.
 */
function titleKey(title: string): string {
  return title.normalize("NFC").toLowerCase();
}

/**
 * freeTitle is the first of titles (Untitled, Untitled 2, …), counting from
 * 1, that no sibling and none of taken has (M4 design 4).
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
