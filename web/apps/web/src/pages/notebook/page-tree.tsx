import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import {
  draggable,
  dropTargetForElements,
  monitorForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import {
  attachInstruction,
  extractInstruction,
  type Instruction,
} from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { ChevronRight, Ellipsis, Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { NavLink, useParams } from "react-router";
import useSWR from "swr";

import { ConfirmDialog } from "../../app/confirm-dialog";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
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
import { usePageTree } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { subtreeOf } from "../../stores/page-tree";
import { useWorkspace } from "../workspace/workspace-layout";
import { MovePageDialog } from "./move-page-dialog";
import { useNewPage } from "./new-page";
import { dropMove, dropOperations } from "./page-drag";
import { RenamePageDialog } from "./rename-page-dialog";

/** writes tells whether the account writes the notebook's pages: its editors and admins, not its readers (PG12). */
export function writes(notebook: Notebook): boolean {
  return notebook.role !== "reader";
}

/** What a drag carries: the page dragged, of which notebook. */
type DragData = { page: string; notebook: string };

function isDragData(data: Record<string | symbol, unknown>): data is DragData {
  return typeof data.page === "string" && typeof data.notebook === "string";
}

/** The tree's means, which each item takes: its notebook, its pages, and where a write's failure shows. */
type TreeContext = {
  notebook: Notebook;
  pages: PageTreeStore;
  heading: RefObject<HTMLHeadingElement | null>;
  newPage: ReturnType<typeof useNewPage>;
  fail: (error: unknown) => void;
};

/**
 * PageTree is the notebook's pages in the left column (M4/P5 design 3.6,
 * 3.7): a navigation of its own, below the workspace's, its pages nested
 * lists of links, each page with children a button that shows or hides
 * them. No ARIA tree: the items are links, which Tab goes through. The
 * page shown is marked, and its ancestors open as it is shown; which pages
 * are open the notebook's store keeps, for the generation.
 *
 * An editor or admin also gets New page, each page's menu (New subpage,
 * Rename, Move to…, Delete) and dragging: before a page, after it, or into
 * it. A drop the tree forbids shows blocked and sends nothing. A write
 * refused, or a creation that gave up, says why below the heading.
 */
export const PageTree = observer(function PageTree({ notebook }: { notebook: Notebook }) {
  const pages = usePageTree(notebook);
  const { pageId } = useParams();
  const t = useT();
  const here = useMounted();
  const heading = useRef<HTMLHeadingElement>(null);
  const newPage = useNewPage(notebook, here);
  const [failure, setFailure] = useState<unknown>();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const read = pages.nodes !== undefined;
  const writer = writes(notebook);
  useEffect(() => {
    if (pageId !== undefined && read) {
      pages.openTo(pageId);
    }
  }, [pages, pageId, read]);
  useEffect(() => {
    if (!writer) {
      return undefined;
    }
    return monitorForElements({
      canMonitor: ({ source }) => isDragData(source.data) && source.data.notebook === notebook.id,
      onDrop: ({ source, location }) => {
        const target = location.current.dropTargets[0];
        const instruction = target === undefined ? null : extractInstruction(target.data);
        const tree = pages.tree;
        if (
          !isDragData(source.data) ||
          target === undefined ||
          instruction === null ||
          instruction.blocked ||
          tree === undefined
        ) {
          return;
        }
        const dragged = source.data.page;
        const move = dropMove(tree, dragged, String(target.data.page), instruction.operation);
        if (move === undefined) {
          return;
        }
        setFailure(undefined);
        pages.move(dragged, move).then(
          () => pages.openTo(dragged),
          (refusal: unknown) => setFailure(refusal)
        );
      },
    });
  }, [writer, notebook.id, pages]);
  const failed = failure === undefined ? newPage.failed : errorText(failure, t);
  const context: TreeContext = { notebook, pages, heading, newPage, fail: setFailure };
  return (
    <nav aria-label={t("page.tree", { notebook: notebook.name })} className="space-y-1">
      <div className="flex items-center justify-between gap-1">
        <h2
          ref={heading}
          tabIndex={-1}
          className="truncate px-3 text-xs font-medium text-muted-foreground outline-none"
        >
          {notebook.name}
        </h2>
        {writer && (
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            aria-label={t("page.new")}
            disabled={newPage.sending}
            onClick={() => void newPage.create(null)}
          >
            <Plus />
          </Button>
        )}
      </div>
      {failed !== undefined && <Alert>{failed}</Alert>}
      {read ? (
        <PageList context={context} parent={null} depth={0} />
      ) : (
        <div className="px-3">
          <NotLoaded error={error} retry={() => void mutate()} />
        </div>
      )}
    </nav>
  );
});

type ListProps = { context: TreeContext; parent: string | null; depth: number; id?: string };

const PageList = observer(function PageList({ context, parent, depth, id }: ListProps) {
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
  const writer = writes(notebook);
  const children = pages.childrenOf(node.id).length > 0;
  const open = children && pages.isOpen(node.id);
  const listId = useId();
  const row = useRef<HTMLDivElement>(null);
  const instruction = useDrag(context, node, row);
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
            aria-label={t("page.subpagesOf", { name: node.name })}
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
        {writer && <PageMenu context={context} node={node} />}
        {instruction !== null && instruction.operation !== "combine" && (
          <span
            aria-hidden
            className={cn(
              "pointer-events-none absolute right-0 left-0 h-0.5",
              instruction.operation === "reorder-before" ? "-top-px" : "-bottom-px",
              instruction.blocked ? "bg-destructive" : "bg-primary"
            )}
          />
        )}
      </div>
      {open && <PageList context={context} parent={node.id} depth={depth + 1} id={listId} />}
    </li>
  );
});

/**
 * useDrag makes the row of node draggable and a drop target, for an editor
 * or admin: the hitbox offers before, after and into it, each blocked where
 * the tree forbids it (page-drag.ts). It answers the drop shown over the
 * row, if any; the tree's monitor moves the page.
 */
function useDrag(context: TreeContext, node: TreeNode, row: RefObject<HTMLDivElement | null>): Instruction | null {
  const { notebook, pages } = context;
  const [instruction, setInstruction] = useState<Instruction | null>(null);
  const writer = writes(notebook);
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
            { input, element, operations: tree === undefined ? {} : dropOperations(tree, dragged, node.id) }
          );
        },
        onDrag: ({ self }) => setInstruction(extractInstruction(self.data)),
        onDragLeave: () => setInstruction(null),
        onDrop: () => setInstruction(null),
      })
    );
  }, [row, writer, node.id, notebook.id, pages]);
  return instruction;
}

type Dialogs = "rename" | "move" | "delete" | undefined;

/**
 * PageMenu is the actions on a page of an editor's or admin's tree: New
 * subpage, Rename, Move to… and Delete, the last three in dialogs the menu
 * holds. A dialog closed gives the focus back to the menu's button; a
 * deletion gives it to the tree's heading, unless the page shown went with
 * it: its shell then goes to the parent, arrived at.
 */
const PageMenu = observer(function PageMenu({ context, node }: { context: TreeContext; node: TreeNode }) {
  const { notebook, pages, heading, newPage, fail } = context;
  const { pageId } = useParams();
  const t = useT();
  const button = useRef<HTMLButtonElement>(null);
  const [dialog, setDialog] = useState<Dialogs>();
  const held = (name: Exclude<Dialogs, undefined>) => ({
    open: dialog === name,
    onOpenChange: (next: boolean) => setDialog(next ? name : undefined),
    onClosed: () => button.current?.focus(),
  });
  const subtree = pages.tree === undefined ? [] : subtreeOf(pages.tree, node.id);
  const shown = subtree.some((page) => page.id === pageId);
  const count = subtree.length - 1;
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            ref={button}
            type="button"
            aria-label={t("page.actions", { name: node.name })}
            className="rounded p-1 text-muted-foreground opacity-0 group-hover:opacity-100 hover:bg-accent focus-visible:opacity-100 data-[state=open]:opacity-100"
          >
            <Ellipsis className="size-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled={newPage.sending} onSelect={() => void newPage.create(node.id)}>
            {t("page.newSubpage")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("rename")}>{t("page.rename")}</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("move")}>{t("page.moveTo")}</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setDialog("delete")}>{t("page.delete")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <RenamePageDialog notebook={notebook} page={node} held={held("rename")} />
      <MovePageDialog notebook={notebook} page={node} held={held("move")} />
      <ConfirmDialog
        held={{
          ...held("delete"),
          onClosed: (deleted) => {
            if (!deleted) {
              button.current?.focus();
            } else if (!shown) {
              heading.current?.focus();
            }
          },
        }}
        title={t("page.deleteTitle", { name: node.name })}
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
      />
    </>
  );
});
