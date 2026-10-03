import { monitorForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { extractInstruction } from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { Plus } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import useSWR from "swr";

import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { useNewPage } from "./new-page";
import { dropMove } from "./page-drag";
import { isDragData, PageList, type TreeContext } from "./page-tree-item";

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
 * it. A drop the tree forbids shows blocked and sends nothing. The last
 * write refused, or a creation that gave up, says why below the heading.
 * A page moved keeps the focus on its menu's button, wherever it went.
 */
export const PageTree = observer(function PageTree({ notebook }: { notebook: Notebook }) {
  const pages = usePageTree(notebook);
  const { pageId } = useParams();
  const t = useT();
  const nav = useRef<HTMLElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const newPage = useNewPage(notebook);
  const [failure, setFailure] = useState<unknown>();
  const [focusing, setFocusing] = useState<string>();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const read = pages.nodes !== undefined;
  const writer = writesPages(notebook.role);
  useEffect(() => {
    if (pageId !== undefined && read) {
      pages.openTo(pageId);
    }
  }, [pages, pageId, read]);
  // The page's item may be another by now: a move to another parent mounts it anew.
  useEffect(() => {
    if (focusing !== undefined) {
      setFocusing(undefined);
      const actions = nav.current?.querySelector<HTMLElement>(`[data-actions-of="${focusing}"]`);
      (actions ?? heading.current)?.focus();
    }
  }, [focusing]);
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
  const create = (parent: string | null) => {
    setFailure(undefined);
    void newPage.create(parent).then(setFailure);
  };
  const context: TreeContext = {
    notebook,
    pages,
    heading,
    newPage: { create, sending: newPage.sending },
    fail: setFailure,
    focusActions: setFocusing,
  };
  return (
    <nav ref={nav} aria-label={t("page.tree", { notebook: notebook.name })} className="space-y-1">
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
            onClick={() => create(null)}
          >
            <Plus />
          </Button>
        )}
      </div>
      {failure !== undefined && <Alert>{errorText(failure, t)}</Alert>}
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
