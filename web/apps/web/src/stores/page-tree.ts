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
