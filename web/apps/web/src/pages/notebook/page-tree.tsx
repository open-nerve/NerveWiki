import { monitorForElements } from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { extractInstruction } from "@atlaskit/pragmatic-drag-and-drop-hitbox/list-item";
import { Book, ChevronDown, History, List, Plus, Settings2 } from "lucide-react";
import { observable, runInAction } from "mobx";
import { observer } from "mobx-react-lite";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Link, NavLink, useMatch, useNavigate, useParams } from "react-router";
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
import { cn } from "../../lib/cn";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree, useStore } from "../../stores/context";
import { useNewPage } from "./new-page";
import { distinctName } from "./distinct-name";
import { dropMove, isDragData } from "./page-drag";
import { boxOf, PageList, TreeMenu, type OpenMenu, type TreeContext } from "./page-tree-item";
import { useSourceMove } from "./source-move-confirmation";

/**
 * What shows the row's cog and the tree's New page (decision 778): a hover
 * of the notebook's row or of either, or the keyboard's focus; always where
 * nothing hovers, and on a narrow screen. A hover of the tree's rows or of
 * the places does not.
 */
const shown =
  "opacity-40 transition-[opacity,color,background-color] duration-150 hover:opacity-100 focus-visible:opacity-100 group-has-[[data-notebook-row]:hover]/notebook:opacity-100 group-has-[[data-notebook-new]:hover]/notebook:opacity-100 [@media(hover:none)]:opacity-100 max-md:opacity-100";

/**
 * PageTree is the notebook's pages in the left column (M4/P5 design 3.6,
 * 3.7): a navigation of its own, in the workspace's, its pages nested
 * lists of links, each page with children a button that shows or hides
 * them. No ARIA tree: the items are links, which Tab goes through. The
 * page shown is marked, and its ancestors open as it is shown; which pages
 * are open the notebook's store keeps, for the generation.
 *
 * Its heading is the notebook's row in the list (M13/P2/S2, decision 778),
 * before the navigation: a link to the notebook's home, then the cog of the
 * notebook's settings; the navigation's New page stands beside the cog, at
 * the row's end, both of them faint until the row is hovered.
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
  const atHome = useMatch(home) !== null;
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
  // New with each render, as close is: a re-read list (another tab's change) moving the notebook within its group
  // renders the tree again, and the menu with it, which measures its button again. A move to the other group is the
  // slot's to tell the menu (useSlotMoves, decision 800).
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
    <>
      <div
        data-notebook-row
        data-home={atHome || undefined}
        className={cn(
          "flex h-7.5 items-stretch rounded-row text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground [@media(hover:hover)]:not-data-home:group-has-[[data-notebook-new]:hover]/notebook:bg-hover [@media(hover:hover)]:not-data-home:group-has-[[data-notebook-new]:hover]/notebook:text-foreground data-home:bg-active data-home:font-medium data-home:text-foreground",
          // Room for the cog and New page, or the cog alone.
          writer ? "pr-14" : "pr-8"
        )}
      >
        <h2 ref={heading} tabIndex={-1} className="flex min-w-0 flex-1 outline-none">
          {/* The whole row's height and its left: the ⌄ only shows the notebook open, as it always is. */}
          <NavLink to={home} end className="flex min-w-0 flex-1 items-center gap-2.25 rounded-row pl-2">
            <ChevronDown aria-hidden className="-mr-1 -ml-0.5 size-3.5 shrink-0 text-faint" />
            <Book aria-hidden className="size-4 shrink-0" />
            <span className="min-w-0 truncate">{notebook.name}</span>
          </NavLink>
        </h2>
        <Link
          to={`${home}/settings/general`}
          aria-label={t("notebook.settingsOf", { notebook: notebook.name })}
          className={cn(
            "absolute top-0.75 grid size-6 place-items-center rounded-row text-faint hover:bg-hover hover:text-foreground [&_svg]:size-4",
            shown,
            writer ? "right-8" : "right-1.5"
          )}
        >
          <Settings2 aria-hidden />
        </Link>
      </div>
      <nav ref={nav} aria-label={t("page.tree", { notebook: notebook.name })} className="flex flex-col">
        {writer && (
          <Button
            data-notebook-new
            variant="ghost"
            size="icon-sm"
            // At the row's end, placed by the notebook's container. Disabled, fainter still, over the opacities that
            // show it, which come after Button's disabled one and would otherwise win (decision 778).
            className={cn("absolute top-0.75 right-1.5 size-6 rounded-row text-faint", shown, "disabled:opacity-25!")}
            aria-label={t("page.new")}
            disabled={newPage.sending}
            onClick={() => create(null)}
          >
            <Plus />
          </Button>
        )}
        {notebook.mode === "llm_wiki" && (
          <div className="space-y-1">
            {/* Opened by the tree, as its pages are (M12/P2/S1). */}
            <Link
              to={`${home}/index`}
              state={opened}
              className="flex items-center gap-2 rounded-md px-3 py-1.5 text-sm hover:bg-hover"
            >
              <List aria-hidden="true" className="size-4" />
              {t("index.title")}
            </Link>
            <Link
              to={`${home}/history`}
              state={opened}
              className="flex items-center gap-2 rounded-md px-3 py-1.5 text-sm hover:bg-hover"
            >
              <History aria-hidden="true" className="size-4" />
              {t("history.title")}
            </Link>
          </div>
        )}
        {failure !== undefined && (
          // The navigation has no space between its rows: the notice keeps clear of the ones above and below it.
          <Alert className="my-1">
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
    </>
  );
});
