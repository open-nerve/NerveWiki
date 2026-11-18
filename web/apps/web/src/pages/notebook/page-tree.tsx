import { monitorForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { extractInstruction } from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { History, List, Plus } from "lucide-react";
import { observable, runInAction } from "mobx";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";
import useSWR from "swr";

import { opened } from "../../app/arrival";
import { writesNotebook } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { pagesLocked } from "../../app/pages-locked";
import { errorText } from "../../app/problem-messages";
import { duplicateText } from "./duplicate-text";
import { useNodeNavigate } from "../node-target";
import { useWorkspace } from "../workspace/workspace-layout";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree, useStore } from "../../stores/context";
import { useNewPage } from "./new-page";
import { distinctName } from "./distinct-name";
import { dropMove, isDragData } from "./page-drag";
import { boxOf, PageList, TreeMenu, type OpenMenu, type TreeContext } from "./page-tree-item";
import { useSourceMove } from "./source-move-confirmation";

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
 * write refused, or a creation that gave up, says why below the heading: a
 * move refused for the pages whose links it would write again being
 * edited names those pages and their editors (M6/P4).
 * A page moved keeps the focus on its menu's button, wherever it went.
 * A deletion's confirmation says how long the trash keeps the page, as
 * the instance has it, read once here for every page's menu (M8/P2 design
 * 4.5).
 *
 * Ten thousand pages are cheap (M12/P4/S4 design 3.2–3.5): a row is a
 * memo of plain values its list computes, which another's change to the
 * tree renders again only where they changed; its link is a plain anchor
 * its list follows; the tree has one menu, opened at a row's button, and
 * a row mounts a dialog only once it is chosen. A list draws its first
 * rows at once and the rest a chunk after each paint (useShownRows), and
 * at once as far as the row the focus goes to.
 */
export const PageTree = observer(function PageTree({ notebook }: { notebook: Notebook }) {
  const pages = usePageTree(notebook);
  const { slug } = useWorkspace();
  const home = `/${slug}/notebooks/${notebook.id}`;
  const navigateNode = useNodeNavigate(pages);
  const { pageId } = useParams();
  const t = useT();
  const { account, instance } = useStore();
  const me = account?.me?.id;
  // Read once here for every page's deletion; each dialog reads it from the store as it renders.
  useSWR("instance", () => instance.load());
  const nav = useRef<HTMLElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const newPage = useNewPage(notebook);
  const [failure, setFailure] = useState<unknown>();
  // The page whose menu button is to have the focus once the tree has it: observable, so that a list not all drawn
  // draws the way to it (M12/P4/S4 design 3.6).
  const [focusingBox] = useState(() => observable.box<string | undefined>(undefined));
  const focusing = focusingBox.get();
  const { error, mutate } = useSWR(["pages", notebook.id], () => pages.load());
  const read = pages.tree !== undefined;
  const writer = writesNotebook(notebook);
  const { move: sourceMove, confirmation } = useSourceMove(pages);
  const navigate = useNavigate();
  const [menu, setMenu] = useState<OpenMenu>();
  const menuId = useId();
  // The notebook as the tree was last given it: where the store no longer finds it, this tab editing one of its
  // pages unsaved, the layout keeps it as it was last found (M5/P4 design 3.9).
  const lastNotebook = useRef(notebook);
  useEffect(() => {
    lastNotebook.current = notebook;
  });
  useEffect(() => {
    if (pageId !== undefined && read) {
      pages.openTo(pageId);
    }
  }, [pages, pageId, read]);
  // The page's item may be another by now: a move to another parent mounts it anew.
  useEffect(() => {
    if (focusing !== undefined) {
      runInAction(() => focusingBox.set(undefined));
      const actions = nav.current?.querySelector<HTMLElement>(`[data-actions-of="${focusing}"]`);
      (actions ?? heading.current)?.focus();
    }
  }, [focusingBox, focusing]);
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
        sourceMove(dragged, move.parent_id, (beforeSend) => pages.move(dragged, move, beforeSend)).then(
          (done) => (done ? pages.openTo(dragged) : undefined),
          (refusal: unknown) => setFailure(refusal)
        );
      },
    });
  }, [writer, notebook.id, pages, sourceMove]);
  const create = (parent: string | null) => {
    setFailure(undefined);
    void newPage.create(parent).then(setFailure);
  };
  // The same for the tree's life, so that the rows' memo holds: what changes as the tree is shown, the lists and
  // dialogs read from the store, or the menu takes as its own (M12/P4/S4 design 3.2).
  const context = useMemo<TreeContext>(
    () => ({
      pages,
      heading,
      home,
      lastNotebook,
      menu: menuId,
      fail: setFailure,
      focusing: focusingBox,
      focusActions: (id) => runInAction(() => focusingBox.set(id)),
      openMenu: (id, button) => setMenu({ id, button: button.id, box: boxOf(button) }),
      left: (id) => setMenu((open) => (open?.id === id ? undefined : open)),
      go: navigate,
      rows: new Map(),
    }),
    [pages, home, menuId, navigate, focusingBox]
  );
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
      {notebook.mode === "llm_wiki" && (
        <div className="space-y-1">
          {/* Opened by the tree, as its pages are (M12/P2/S1). */}
          <Link
            to={`${home}/index`}
            state={opened}
            className="flex items-center gap-2 rounded-md px-3 py-1.5 text-sm hover:bg-accent"
          >
            <List aria-hidden="true" className="size-4" />
            {t("index.title")}
          </Link>
          <Link
            to={`${home}/history`}
            state={opened}
            className="flex items-center gap-2 rounded-md px-3 py-1.5 text-sm hover:bg-accent"
          >
            <History aria-hidden="true" className="size-4" />
            {t("history.title")}
          </Link>
        </div>
      )}
      {failure !== undefined && (
        <Alert>
          {duplicateText(failure, pages, (id) => {
            setFailure(undefined);
            navigateNode(id);
          }) ??
            pagesLocked(failure, t, me, (id) => distinctName(pages.tree, id, t)) ??
            errorText(failure, t)}
        </Alert>
      )}
      {read ? (
        <PageList context={context} parent={null} depth={0} />
      ) : (
        <div className="px-3">
          <NotLoaded error={error} retry={() => void mutate()} />
        </div>
      )}
      {writer && read && (
        <TreeMenu
          context={context}
          nav={nav}
          menu={menu}
          close={() => setMenu(undefined)}
          create={create}
          sending={newPage.sending}
        />
      )}
      {confirmation}
    </nav>
  );
});
