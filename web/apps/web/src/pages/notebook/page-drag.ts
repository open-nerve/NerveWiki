import type { NodeMove } from "../../services/page.service";
import { canHold, childrenOf, type TreeIndex } from "../../stores/page-tree";

/** A drop's operation on a page, as the list-item hitbox names it: before it, after it, or into it. */
export type DropOperation = "reorder-before" | "reorder-after" | "combine";

const operations: readonly DropOperation[] = ["reorder-before", "reorder-after", "combine"];

/**
 * dropMove is the move that dropping the page dragged on the page target
 * with operation asks for (M4/P5 design 3.7), or none when the tree
 * forbids it: onto the page itself, into its own subtree, or deeper than
 * maxDepth. Before a page is right after its sibling before it, other than
 * the page dragged (none: first); after a page is right after it; into a
 * page is its last child.
 */
export function dropMove(
  tree: TreeIndex,
  dragged: string,
  target: string,
  operation: DropOperation
): NodeMove | undefined {
  const page = tree.byId.get(target);
  if (page === undefined || target === dragged) {
    return undefined;
  }
  const parent = operation === "combine" ? target : page.parent_id;
  if (!canHold(tree, parent, dragged)) {
    return undefined;
  }
  switch (operation) {
    case "combine":
      return { parent_id: target };
    case "reorder-after":
      return { parent_id: parent, after_id: target };
    case "reorder-before": {
      const siblings = childrenOf(tree, parent).filter((sibling) => sibling.id !== dragged);
      const before = siblings[siblings.findIndex((sibling) => sibling.id === target) - 1];
      return { parent_id: parent, after_id: before?.id ?? null };
    }
  }
}

/**
 * dropOperations is what the hitbox offers over target while dragged is
 * dragged: each operation available, or blocked where the tree forbids it,
 * which shows without a request; none over the page itself.
 */
export function dropOperations(
  tree: TreeIndex,
  dragged: string,
  target: string
): Record<DropOperation, "available" | "blocked" | "not-available"> {
  return Object.fromEntries(
    operations.map((operation) => [
      operation,
      target === dragged
        ? "not-available"
        : dropMove(tree, dragged, target, operation) === undefined
          ? "blocked"
          : "available",
    ])
  ) as Record<DropOperation, "available" | "blocked" | "not-available">;
}
