import { reaction } from "mobx";
import { observer } from "mobx-react-lite";
import { useContext, useEffect, useLayoutEffect, useRef, type RefObject } from "react";
import { useLocation, useNavigate } from "react-router";

import { arrived } from "../../app/arrival";
import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { enhance, Enhancements } from "../../reading/enhancement";
import { taskText } from "../../reading/task-toggle";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { useT } from "../../i18n/i18n";
import { useAssets, usePageTree, useStore } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";
import { usePageView } from "./page-view";
import { watchReader } from "./readers-input";
import { useUnresolvedLinks } from "./unresolved-link";

/**
 * ReadingView is the page's content as the server renders it (M4/P5 design
 * 3.8): HTML the server sanitized, which goes into the article as it is.
 * It is read by page, and read again as SWR does (a refocus, a retry):
 * another's write then shows. A 503 server_busy is read again after its
 * Retry-After.
 *
 * Once the HTML is in, the app's enhancements run on it in their order;
 * before the HTML is replaced, and when the view goes, they are undone in
 * the reverse order (reading/enhancement.ts). They run again as the
 * language changes, their names in it: the reader changes it in a menu,
 * which has the focus. Not as the theme does, the system's maybe as one
 * reads: those that draw in it follow it (onThemeChange). A writer's
 * tick task items through it, one of the page's at a time (the page tree store's
 * oneToggle), and refused tells the page what was refused, undefined as a
 * toggle starts (M5/P6 design 3.5). A task item's checkbox that had the
 * focus as the HTML is replaced has it back in the new HTML, without a
 * scroll, while the box at its position is the same item's (its text, and
 * its state, or the state the view's own toggle asked for): a tick does
 * not move it, a write that moved the items does. A folded callout it is
 * in opens again, as the reader had it.
 *
 * An enhancement goes to another address through the router: to a page,
 * whose heading takes the focus; or to an anchor. An address with an
 * anchor has the view go to the element it names, once the HTML is in,
 * once for each time the app goes to the address: the element shows and
 * takes the focus (M6/P3 design 6.7). The page keeps the last navigation
 * its view took in (anchored), as the view goes while the page is edited:
 * coming back from editing goes nowhere. When the anchor names no element
 * as the page opens (its first navigation, by a load or from another
 * page), unanchored gives the page's heading the focus if it is nowhere
 * (what had it went with the page before); and a view from the cache, which
 * may be older than the link, is read again: the element it brings shows
 * and takes the focus, if the reader has done nothing since (scrolled,
 * clicked, touched, pressed a key: the browser's own scrolling is none of
 * these, nor are an assistive technology's moves) and the focus is where it
 * was; a read after that one moves nothing. A link of the page to no
 * element leaves the focus where it is, and the page. An element with an
 * id that had the focus as the HTML is replaced, the anchor's among them,
 * has it back in the new HTML, shown again if it showed and no longer
 * does: a view read again stays where it is.
 *
 * A link to a page that is not there, acted on, opens the view's dialog
 * (unresolved-link.tsx): a writer may create the page, where the server
 * says it would go (M6/P6 design 7).
 */
export const ReadingView = observer(function ReadingView({
  notebook,
  page,
  refused,
  unanchored,
  anchored,
}: {
  notebook: Notebook;
  page: TreeNode;
  refused: (error: unknown) => void;
  unanchored: () => void;
  // The last navigation the page's view took in, its anchor gone to: the location's key and fragment (one key may
  // have several: an address typed in).
  anchored: RefObject<string | undefined>;
}) {
  const { slug } = useWorkspace();
  const pages = usePageTree(notebook);
  const assets = useAssets(notebook);
  const enhancements = useContext(Enhancements);
  const t = useT();
  const { preferences } = useStore();
  const navigate = useNavigate();
  const location = useLocation();
  // The element with an id focused as the HTML was replaced, and whether it showed.
  const focusedTarget = useRef<{ id: string; shown: boolean } | undefined>(undefined);
  // While the view from the cache is read again for the anchor the page opened at, which named no element of it:
  // what had the focus then, and how the wait ends.
  const awaited = useRef<{ focus: Element | null; end: () => void } | undefined>(undefined);
  const { data, error, mutate } = usePageView(notebook, page.id);
  // Whether the view came from the cache: older, maybe, than the address.
  const cached = useRef(data !== undefined);
  const article = useRef<HTMLElement>(null);
  // The task item focused as the HTML was replaced: its position, its text and its state.
  const focusedTask = useRef<{ task: string; text: string; checked: boolean } | undefined>(undefined);
  // The view's own toggle, from its sending: the item's position and the state asked for, until a view shows it
  // or the toggle fails. Meanwhile the item may come in either state: another's write, read again, may come first.
  const toggled = useRef<{ task: string; checked: boolean } | undefined>(undefined);
  // The page's latest refused: the HTML is not replaced for a new one.
  const latestRefused = useRef(refused);
  const latestUnanchored = useRef(unanchored);
  const links = useUnresolvedLinks({
    notebook,
    page: page.id,
    pages,
    article,
    reload: () => void mutate(),
    report: (failure) => latestRefused.current(failure),
  });
  const latestUnresolved = useRef(links.unresolved);
  useEffect(() => {
    latestRefused.current = refused;
    latestUnanchored.current = unanchored;
    latestUnresolved.current = links.unresolved;
  });
  const html = data?.html;
  const revision = data?.revision;
  const assetsExpire = data?.assets_expire_at ?? null;
  const { id: notebookId, role } = notebook;
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined || revision === undefined) {
      return undefined;
    }
    container.innerHTML = html;
    const undo = enhance(enhancements, container, {
      workspace: slug,
      notebook: notebookId,
      page: page.id,
      revision,
      role,
      t,
      theme: () => preferences.resolvedTheme,
      onThemeChange: (listener) => reaction(() => preferences.resolvedTheme, listener),
      reload: () => void mutate(),
      // An address without an anchor arrives at the page, whose heading takes the focus: the link had it.
      navigate: (to) => void navigate(to, to.includes("#") ? undefined : { state: arrived }),
      toggleTask: writesPages(role)
        ? async (offset, checked) => {
            await pages.oneToggle(page.id, async () => {
              latestRefused.current(undefined);
              toggled.current = { task: offset.toString(), checked };
              try {
                await pages.toggleTask(page.id, { base_revision: revision, offset, checked });
              } catch (failure) {
                toggled.current = undefined;
                if (failure instanceof ApiError && failure.code === "page.revision_mismatch") {
                  await mutate();
                }
                throw failure;
              }
              await mutate();
            });
          }
        : undefined,
      report: (failure) => latestRefused.current(failure),
      unresolved: (link) => void latestUnresolved.current(link),
      assetsExpire,
      assetAddress: (id) => assets.address(id),
    });
    const focused = focusedTask.current;
    const asked = toggled.current;
    focusedTask.current = undefined;
    const at = (task: string) => container.querySelector<HTMLElement>(`input[data-task="${CSS.escape(task)}"]`);
    if (asked !== undefined && at(asked.task)?.hasAttribute("checked") === asked.checked) {
      toggled.current = undefined;
    }
    const box = focused && at(focused.task);
    // The same item: its text, and its state, or the one the view's own toggle asked for.
    const state = box?.hasAttribute("checked");
    if (
      box &&
      taskText(box) === focused.text &&
      (state === focused.checked || (asked?.task === focused.task && state === asked.checked))
    ) {
      unfold(box);
      box.focus({ preventScroll: true });
    }
    const target = focusedTarget.current;
    focusedTarget.current = undefined;
    const again = target && byId(container, target.id);
    if (again) {
      // Scrolled only when it no longer shows: a scroll would reset a wide content's too.
      focusOn(again, target.shown && !shows(again) ? { block: "nearest" } : undefined);
    }
    return () => {
      // Undone, a checkbox is disabled again, which HTML's focus fixup takes the focus from: Chromium at the next
      // rendering, an engine that applies the rule at once before the new HTML is in. Which had it is read first.
      const active = document.activeElement;
      focusedTask.current =
        active instanceof HTMLInputElement && active.dataset.task !== undefined && container.contains(active)
          ? { task: active.dataset.task, text: taskText(active), checked: active.hasAttribute("checked") }
          : undefined;
      focusedTarget.current =
        active instanceof HTMLElement && active.id !== "" && container.contains(active)
          ? { id: active.id, shown: shows(active) }
          : undefined;
      undo();
    };
  }, [
    html,
    revision,
    assetsExpire,
    enhancements,
    slug,
    notebookId,
    role,
    t,
    preferences,
    page.id,
    mutate,
    pages,
    assets,
    navigate,
  ]);
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined) {
      return;
    }
    const navigation = location.key + location.hash;
    const anchor = location.hash.slice(1);
    const target = anchor === "" ? undefined : named(container, anchor);
    if (anchored.current === navigation) {
      // The view read again for it has the element, the reader having done nothing, the focus where it was.
      const waiting = awaited.current;
      if (waiting !== undefined && target !== undefined && document.activeElement === waiting.focus) {
        waiting.end();
        focusOn(target, {});
      }
      return;
    }
    // The page's first navigation: it opened, by a load or from another page.
    const opened = anchored.current === undefined;
    anchored.current = navigation;
    awaited.current?.end();
    if (target !== undefined) {
      focusOn(target, {});
      return;
    }
    if (anchor === "" || !opened) {
      return;
    }
    if (document.activeElement === document.body) {
      latestUnanchored.current();
    }
    if (cached.current) {
      // Once per page: the wait ends with this read, at the next navigation, or as the reader does something. Not as
      // the view goes: StrictMode's second mount would end it at once; the page left, its read settling ends it.
      const reader = watchReader({ acts: () => end() });
      const end = () => {
        reader.end();
        if (awaited.current?.end === end) {
          awaited.current = undefined;
        }
      };
      awaited.current = { focus: document.activeElement, end };
      void mutate().finally(end);
    }
  }, [anchored, html, location.hash, location.key, mutate]);
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  // Named by the page, a landmark: what is wider than it scrolls in its own region (reading/scroll-regions.ts).
  return (
    <>
      <article ref={article} aria-label={page.name} className="nw-reading min-w-0" />
      {links.dialog}
    </>
  );
});

/** named is the element of container whose id the address's fragment is, decoded, if there is one. */
function named(container: HTMLElement, fragment: string): HTMLElement | undefined {
  let id = fragment;
  try {
    id = decodeURIComponent(fragment);
  } catch {
    // Not an escape: the fragment as it is.
  }
  return byId(container, id);
}

/** byId is the element of container whose id is id, if there is one. */
function byId(container: HTMLElement, id: string): HTMLElement | undefined {
  return container.querySelector<HTMLElement>(`[id="${CSS.escape(id)}"]`) ?? undefined;
}

/** shows tells whether element is in the window's view. */
function shows(element: HTMLElement): boolean {
  const { top, bottom } = element.getBoundingClientRect();
  return bottom > 0 && top < window.innerHeight;
}

/**
 * focusOn gives element the focus, focusable as an anchor's target is, scrolled into view as show says, if it does;
 * a folded callout it is in opens first.
 */
function focusOn(element: HTMLElement, show: ScrollIntoViewOptions | undefined) {
  unfold(element);
  if (!element.hasAttribute("tabindex")) {
    element.setAttribute("tabindex", "-1");
  }
  if (show !== undefined) {
    element.scrollIntoView(show);
  }
  element.focus({ preventScroll: true });
}

/** unfold opens the folded callouts (closed details) element is in, which could show nothing of it otherwise. */
function unfold(element: HTMLElement) {
  for (let parent = element.parentElement; parent !== null; parent = parent.parentElement) {
    if (parent instanceof HTMLDetailsElement && !parent.open) {
      parent.open = true;
    }
  }
}
