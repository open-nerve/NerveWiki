import { createContext, use, useLayoutEffect, useRef, useSyncExternalStore } from "react";

import { intoColumn } from "../../app/shell-sidebar";

/**
 * Where the workspace's shell keeps each notebook's part of the left column (M13/P2/S2, decisions 776, 798, 800): the
 * notebook's row, its page tree and its places, which the notebook's shell portals into the notebook's container and
 * the list's item of the notebook of the address holds. The container is the same element as the notebook moves from
 * one group to the other: the tree is not mounted anew, its dialogs and the rows it drew stay.
 */
type Slot = {
  /** The container of the notebook id's row, tree and places: the same element for the id, each id's kept (798). */
  containerFor: (id: string) => HTMLElement;
  /** Where the focus was in the container as its holder went, to give back once the next one holds it. */
  keepFocus: (id: string, element: HTMLElement | undefined) => void;
  takeFocus: (id: string) => HTMLElement | undefined;
  /** A holder moved a container (to the other group, or out of the fallback place): the tree menu measures again (800). */
  moved: () => void;
  moves: () => number;
  subscribe: (listener: () => void) => () => void;
};

/**
 * makeSlot is a workspace shell's slot, for its life. Each notebook's container is made the first time it is asked for
 * and kept, never dropped as another's is asked for: a render left for another notebook and given up, the notebook's
 * shell rendering again for the first, finds its container still, not a new one out of the document (decision 798).
 * An empty div is cheap, one at most for each notebook a shell shows.
 */
export function makeSlot(): Slot {
  const containers = new Map<string, HTMLElement>();
  const focused = new Map<string, HTMLElement>();
  const listeners = new Set<() => void>();
  let moves = 0;
  return {
    containerFor: (id) => {
      const kept = containers.get(id);
      if (kept !== undefined) {
        return kept;
      }
      const element = document.createElement("div");
      // Relative: the row's cog and the tree's New page stand at its top right, the row's hover shows them.
      element.className = "group/notebook relative";
      containers.set(id, element);
      return element;
    },
    keepFocus: (id, element) => {
      if (element === undefined) {
        focused.delete(id);
      } else {
        focused.set(id, element);
      }
    },
    takeFocus: (id) => {
      const element = focused.get(id);
      focused.delete(id);
      return element;
    },
    moved: () => {
      moves += 1;
      for (const listener of listeners) {
        listener();
      }
    },
    moves: () => moves,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
  };
}

/** Outside a workspace's shell: a slot that never moves. */
const still: Slot = {
  containerFor: () => document.createElement("div"),
  keepFocus: () => undefined,
  takeFocus: () => undefined,
  moved: () => undefined,
  moves: () => 0,
  subscribe: () => () => undefined,
};

/** NotebookSlot is the workspace shell's slot (WorkspaceLayout gives it). */
export const NotebookSlot = createContext<Slot>(still);

/** useNotebookSlot is the slot a notebook's shell portals its part of the left column into. */
export function useNotebookSlot(): Slot {
  return use(NotebookSlot);
}

/**
 * useSlotMoves renders its component again as a holder moves a container: the tree's menu, which measures its page's
 * button after each commit of its own (decision 800). What it answers is a count, of no use but to change.
 */
export function useSlotMoves(): number {
  const slot = use(NotebookSlot);
  return useSyncExternalStore(slot.subscribe, slot.moves);
}

/**
 * SlotHolder holds the container of the notebook id: in the list's item of the notebook of the address, or after the
 * groups for one the list no longer has, kept while a page of it is edited unsaved (NotebookNav). As it goes, it
 * keeps where the focus was in the container: the DOM's removal of the item, which comes after, would give the
 * focus to the body; the next holder of the id, once it holds the container, gives it back if no one has taken it
 * since, and brings it into the column's view (intoColumn), the container having moved (the other group is below).
 */
export function SlotHolder({ id }: { id: string }) {
  const slot = use(NotebookSlot);
  const holder = useRef<HTMLDivElement>(null);
  // WorkspaceLayout draws the column's portal before its Outlet, and :slug loads lazily, Layout having the column's
  // place a commit before: in the commit that mounts both, this layout effect runs before the tree's, whose effects
  // find the container in the document (decision 776).
  useLayoutEffect(() => {
    const at = holder.current;
    if (at === null) {
      return undefined;
    }
    const element = slot.containerFor(id);
    const focused = slot.takeFocus(id);
    // Held before: by a holder gone, its div out of the document. A first holding tells no one (decision 800).
    const moving = element.parentElement !== null;
    at.append(element);
    if (moving) {
      slot.moved();
    }
    if (focused?.isConnected && (document.activeElement === null || document.activeElement === document.body)) {
      focused.focus({ preventScroll: true });
      intoColumn(focused);
    }
    return () => {
      const active = document.activeElement;
      slot.keepFocus(id, active instanceof HTMLElement && element.contains(active) ? active : undefined);
    };
  }, [slot, id]);
  return <div ref={holder} />;
}
