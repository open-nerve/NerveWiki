import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import { draggable, dropTargetForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import {
  attachInstruction,
  extractInstruction,
  type Instruction,
} from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { useEffect, useState, type RefObject } from "react";

import { canMoveNode } from "../../app/source-permissions";
import type { NodeMove } from "../../services/page.service";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { canHold, childrenOf, type TreeIndex } from "../../stores/page-tree";

/** What a drag carries: the page dragged, of which notebook. */
export type DragData = { page: string; notebook: string };

export function isDragData(data: Record<string | symbol, unknown>): data is DragData {
  return typeof data.page === "string" && typeof data.notebook === "string";
}

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
 * which shows without a request; none over the page itself. After target
 * is not offered while it is open with its children shown: the line would
 * stand above its first child, but the page would go after its whole
 * subtree. Into it, or before the next page, says where.
 */
export function dropOperations(
  tree: TreeIndex,
  dragged: string,
  target: string,
  open: boolean,
  permits: (move: NodeMove) => boolean = () => true
): Record<DropOperation, "available" | "blocked" | "not-available"> {
  return Object.fromEntries(
    operations.map((operation) => {
      const move = dropMove(tree, dragged, target, operation);
      return [
        operation,
        target === dragged || (operation === "reorder-after" && open)
          ? "not-available"
          : move === undefined || !permits(move)
            ? "blocked"
            : "available",
      ];
    })
  ) as Record<DropOperation, "available" | "blocked" | "not-available">;
}

/**
 * useDrag makes the row of the page id a drop target for an editor or
 * admin (writer), and draggable when its mode permits moving it (canDrag):
 * the hitbox offers before, after and into it, each blocked where the tree
 * forbids it, and not after it while it shows its children (open). It
 * answers the drop shown over the row, if any; the tree's monitor moves
 * the page. A row of the tree calls it once each time it renders, a
 * reader's too (M12/P4/S4 design 4.1: the guard counts the rows rendered
 * by it).
 */
export function useDrag(
  pages: PageTreeStore,
  id: string,
  row: RefObject<HTMLDivElement | null>,
  open: boolean,
  canDrag: boolean,
  writer: boolean
): Instruction | null {
  const [instruction, setInstruction] = useState<Instruction | null>(null);
  useEffect(() => {
    const element = row.current;
    if (element === null || !writer) {
      return undefined;
    }
    const data: DragData = { page: id, notebook: pages.notebookId };
    return combine(
      ...(canDrag ? [draggable({ element, getInitialData: () => data })] : []),
      dropTargetForElements({
        element,
        canDrop: ({ source }) => isDragData(source.data) && source.data.notebook === pages.notebookId,
        getData: ({ input, source }) => {
          const tree = pages.tree;
          const dragged = isDragData(source.data) ? source.data.page : "";
          return attachInstruction(
            { ...data },
            {
              input,
              element,
              operations:
                tree === undefined
                  ? {}
                  : dropOperations(tree, dragged, id, open, (move) => canMoveNode(pages, dragged, move.parent_id)),
            }
          );
        },
        onDrag: ({ self }) => setInstruction(extractInstruction(self.data)),
        onDragLeave: () => setInstruction(null),
        onDrop: () => setInstruction(null),
      })
    );
  }, [row, writer, id, pages, open, canDrag]);
  return instruction;
}
