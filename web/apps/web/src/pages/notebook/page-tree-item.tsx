import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import { draggable, dropTargetForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import {
  attachInstruction,
  extractInstruction,
  type Instruction,
} from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { ChevronRight, Ellipsis } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { NavLink, useParams } from "react-router";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { writesPages } from "../../app/effective-role";
import { lockedText } from "../../app/problem-messages";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../../components/ui/dropdown-menu";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { depthOf, maxDepth, placeOfTitle, subtreeOf } from "../../stores/page-tree";
import { useWorkspace } from "../workspace/workspace-layout";
import { MovePageDialog } from "./move-page-dialog";
import { dropOperations } from "./page-drag";
import { RenamePageDialog } from "./rename-page-dialog";

// The items of a notebook's page tree (M4/P5 design 3.6, 3.7): each page's
// row, with its link, its children's button, its menu and dialogs, and
// dragging. page-tree.tsx is the tree around them.

/** What a drag carries: the page dragged, of which notebook. */
export type DragData = { page: string; notebook: string };

export function isDragData(data: Record<string | symbol, unknown>): data is DragData {
  return typeof data.page === "string" && typeof data.notebook === "string";
}

/**
 * The tree's means, which each item takes: its notebook, its pages, its
 * heading, the creation of a page under a parent, where a write's failure
 * shows, and the focus given to a page's menu button once the tree has it.
 */
export type TreeContext = {
  notebook: Notebook;
  pages: PageTreeStore;
  heading: RefObject<HTMLHeadingElement | null>;
  newPage: { create: (parent: string | null) => void; sending: boolean };
  fail: (error: unknown) => void;
  focusActions: (id: string) => void;
};

type ListProps = { context: TreeContext; parent: string | null; depth: number; id?: string };

export const PageList = observer(function PageList({ context, parent, depth, id }: ListProps) {
  return (
    <ul id={id} className="flex flex-col gap-0.5">
      {context.pages.childrenOf(parent).map((node) => (
        <PageItem key={node.id} context={context} node={node} depth={depth} />
      ))}
    </ul>
  );
});

type ItemProps = { context: TreeContext; node: TreeNode; depth: number };

const PageItem = observer(function PageItem({ context, node, depth }: ItemProps) {
  const { notebook, pages } = context;
  const { slug } = useWorkspace();
  const t = useT();
  const name = useDistinctName(context, node);
  const writer = writesPages(notebook.role);
  const children = pages.childrenOf(node.id).length > 0;
  const open = children && pages.isOpen(node.id);
  const listId = useId();
  const row = useRef<HTMLDivElement>(null);
  const instruction = useDrag(context, node, row, open);
  return (
    <li>
      <div
        ref={row}
        className={cn(
          "group relative flex items-center gap-0.5 rounded-md",
          instruction?.operation === "combine" &&
            (instruction.blocked ? "ring-2 ring-destructive" : "ring-2 ring-primary")
        )}
        style={{ paddingLeft: `${depth * 0.75}rem` }}
      >
        {children ? (
          <button
            type="button"
            aria-expanded={open}
            aria-controls={open ? listId : undefined}
            aria-label={t("page.subpagesOf", { name })}
            onClick={() => pages.toggle(node.id)}
            className="rounded p-1 text-muted-foreground hover:bg-accent"
          >
            <ChevronRight className={cn("size-4 transition-transform", open && "rotate-90")} />
          </button>
        ) : (
          <span className="w-6 shrink-0" />
        )}
        <NavLink
          to={`/${slug}/notebooks/${notebook.id}/pages/${node.id}`}
          // A link drags itself: a drag from the title would not be the row's.
          draggable={writer ? false : undefined}
          className={({ isActive }) =>
            cn(
              "min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-sm hover:bg-accent",
              isActive ? "bg-accent font-medium" : "text-muted-foreground"
            )
          }
        >
          {node.name}
        </NavLink>
        {writer && <PageMenu context={context} node={node} name={name} />}
        {instruction !== null && instruction.operation !== "combine" && (
          <span
            aria-hidden
            className={cn(
              "pointer-events-none absolute right-0 h-0.5",
              instruction.operation === "reorder-before" ? "-top-px" : "-bottom-px",
              instruction.blocked ? "bg-destructive" : "bg-primary"
            )}
            // At the page's level: where its title starts.
            style={{ left: `${depth * 0.75 + 1.5}rem` }}
          />
        )}
      </div>
      {open && <PageList context={context} parent={node.id} depth={depth + 1} id={listId} />}
    </li>
  );
});

/**
 * useDistinctName is how the controls and dialogs of node's item name it:
 * its title, and where another page of the notebook has the same, also
 * the pages it is under, or the notebook at the root (v0.1 design 13.2,
 * item 17).
 */
function useDistinctName({ notebook, pages }: TreeContext, node: TreeNode): string {
  const t = useT();
  const tree = pages.tree;
  const place = tree === undefined ? undefined : placeOfTitle(tree, node.id);
  return place === undefined ? node.name : t("page.nameIn", { name: node.name, place: place || notebook.name });
}

/**
 * useDrag makes the row of node draggable and a drop target, for an editor
 * or admin: the hitbox offers before, after and into it, each blocked where
 * the tree forbids it, and not after it while it shows its children
 * (page-drag.ts). It answers the drop shown over the row, if any; the
 * tree's monitor moves the page.
 */
function useDrag(
  context: TreeContext,
  node: TreeNode,
  row: RefObject<HTMLDivElement | null>,
  open: boolean
): Instruction | null {
  const { notebook, pages } = context;
  const [instruction, setInstruction] = useState<Instruction | null>(null);
  const writer = writesPages(notebook.role);
  useEffect(() => {
    const element = row.current;
    if (element === null || !writer) {
      return undefined;
    }
    const data: DragData = { page: node.id, notebook: notebook.id };
    return combine(
      draggable({ element, getInitialData: () => data }),
      dropTargetForElements({
        element,
        canDrop: ({ source }) => isDragData(source.data) && source.data.notebook === notebook.id,
        getData: ({ input, source }) => {
          const tree = pages.tree;
          const dragged = isDragData(source.data) ? source.data.page : "";
          return attachInstruction(
            { ...data },
            { input, element, operations: tree === undefined ? {} : dropOperations(tree, dragged, node.id, open) }
          );
        },
        onDrag: ({ self }) => setInstruction(extractInstruction(self.data)),
        onDragLeave: () => setInstruction(null),
        onDrop: () => setInstruction(null),
      })
    );
  }, [row, writer, node.id, notebook.id, pages, open]);
  return instruction;
}

type Dialogs = "rename" | "move" | "delete" | undefined;

/**
 * PageMenu is the actions on a page of an editor's or admin's tree: New
 * subpage, Rename, Move to… and Delete, the last three in dialogs the menu
 * holds. A dialog closed gives the focus back to the menu's button, the
 * page's wherever it moved; a deletion gives it to the tree's heading,
 * unless the page shown went with it: its shell then goes to the parent,
 * arrived at. A page ten levels down has no New subpage. They name the
 * page by name, which tells it from others of its title.
 */
const PageMenu = observer(function PageMenu({
  context,
  node,
  name,
}: {
  context: TreeContext;
  node: TreeNode;
  name: string;
}) {
  const { notebook, pages, heading, newPage, fail, focusActions } = context;
  const { pageId } = useParams();
  const t = useT();
  const me = useStore().account?.me?.id;
  const [dialog, setDialog] = useState<Dialogs>();
  const held = (which: Exclude<Dialogs, undefined>) => ({
    open: dialog === which,
    onOpenChange: (next: boolean) => setDialog(next ? which : undefined),
    onClosed: () => focusActions(node.id),
  });
  const tree = pages.tree;
  const subtree = tree === undefined ? [] : subtreeOf(tree, node.id);
  const deepest = tree !== undefined && depthOf(tree, node.id) >= maxDepth;
  const shown = subtree.some((page) => page.id === pageId);
  const count = subtree.length - 1;
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            data-actions-of={node.id}
            aria-label={t("page.actions", { name })}
            // Shown on hover, or always where a pointer is a finger, which does not hover.
            className="rounded p-1 text-muted-foreground opacity-0 group-hover:opacity-100 hover:bg-accent focus-visible:opacity-100 data-[state=open]:opacity-100 any-pointer-coarse:opacity-100"
          >
            <Ellipsis className="size-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled={newPage.sending || deepest} onSelect={() => newPage.create(node.id)}>
            {t("page.newSubpage")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("rename")}>{t("page.rename")}</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("move")}>{t("page.moveTo")}</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("delete")}>{t("page.delete")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <RenamePageDialog notebook={notebook} page={node} name={name} held={held("rename")} />
      <MovePageDialog notebook={notebook} page={node} name={name} held={held("move")} />
      <ConfirmDialog
        held={{
          ...held("delete"),
          // The store tells whether the page went: once deleted it leaves the tree, and this
          // item with it, before the dialog knows that confirm went through.
          onClosed: () => {
            if (pages.removedTo(node.id) === undefined) {
              focusActions(node.id);
            } else if (!shown) {
              heading.current?.focus();
            }
          },
        }}
        title={t("page.deleteTitle", { name })}
        description={
          count === 0 ? t("page.deleteBody") : `${t("page.deleteBody")} ${t("page.deleteSubpages", { count })}`
        }
        confirmLabel={t("page.delete")}
        sendingLabel={t("page.deleting")}
        cancelLabel={t("page.cancel")}
        confirm={async () => {
          fail(undefined);
          await pages.remove(node.id);
        }}
        // Someone editing the page or one under it refuses the deletion: the dialog names them, and the page.
        explain={(error) => lockedText(error, t, me, (id) => pages.byId(id)?.name)}
      />
    </>
  );
});
