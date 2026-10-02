import { ChevronRight } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useId } from "react";
import { NavLink, useParams } from "react-router";
import useSWR from "swr";

import { NotLoaded } from "../../app/not-loaded";
import { useT } from "../../i18n/i18n";
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * PageTree is the notebook's pages in the left column (M4/P5 design 3.6):
 * a navigation of its own, below the workspace's, its pages nested lists
 * of links, each page with children a button that shows or hides them.
 * No ARIA tree: the items are links, which Tab goes through. The page
 * shown is marked, and its ancestors open as it is shown; which pages are
 * open the notebook's store keeps, for the generation.
 */
export const PageTree = observer(function PageTree({ notebook }: { notebook: Notebook }) {
  const pages = usePageTree(notebook);
  const { pageId } = useParams();
  const t = useT();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const read = pages.nodes !== undefined;
  useEffect(() => {
    if (pageId !== undefined && read) {
      pages.openTo(pageId);
    }
  }, [pages, pageId, read]);
  return (
    <nav aria-label={t("page.tree", { notebook: notebook.name })} className="space-y-1">
      <h2 className="truncate px-3 text-xs font-medium text-muted-foreground">{notebook.name}</h2>
      {read ? (
        <PageList pages={pages} notebook={notebook} parent={null} depth={0} />
      ) : (
        <div className="px-3">
          <NotLoaded error={error} retry={() => void mutate()} />
        </div>
      )}
    </nav>
  );
});

type ListProps = { pages: PageTreeStore; notebook: Notebook; parent: string | null; depth: number; id?: string };

const PageList = observer(function PageList({ pages, notebook, parent, depth, id }: ListProps) {
  return (
    <ul id={id} className="flex flex-col gap-0.5">
      {pages.childrenOf(parent).map((node) => (
        <PageItem key={node.id} pages={pages} notebook={notebook} node={node} depth={depth} />
      ))}
    </ul>
  );
});

type ItemProps = { pages: PageTreeStore; notebook: Notebook; node: TreeNode; depth: number };

const PageItem = observer(function PageItem({ pages, notebook, node, depth }: ItemProps) {
  const { slug } = useWorkspace();
  const t = useT();
  const children = pages.childrenOf(node.id).length > 0;
  const open = children && pages.isOpen(node.id);
  const listId = useId();
  return (
    <li>
      <div className="flex items-center gap-0.5" style={{ paddingLeft: `${depth * 0.75}rem` }}>
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
          className={({ isActive }) =>
            cn(
              "min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-sm hover:bg-accent",
              isActive ? "bg-accent font-medium" : "text-muted-foreground"
            )
          }
        >
          {node.name}
        </NavLink>
      </div>
      {open && <PageList pages={pages} notebook={notebook} parent={node.id} depth={depth + 1} id={listId} />}
    </li>
  );
});
