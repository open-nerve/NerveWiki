import { observer } from "mobx-react-lite";
import { useContext, useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { useLocation, useNavigate } from "react-router";
import useSWR from "swr";

import { arrived } from "../../app/arrival";
import { writesPages } from "../../app/effective-role";
import { NotLoaded } from "../../app/not-loaded";
import { enhance, Enhancements } from "../../reading/enhancement";
import { taskText } from "../../reading/task-toggle";
import { ApiError } from "../../services/api";
import type { Notebook } from "../../services/notebook.service";
import type { TreeNode } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import { useWorkspace } from "../workspace/workspace-layout";

/**
 * ReadingView is the page's content as the server renders it (M4/P5 design
 * 3.8): HTML the server sanitized, which goes into the article as it is.
 * It is read by page, and read again as SWR does (a refocus, a retry):
 * another's write then shows. A 503 server_busy is read again after its
 * Retry-After.
 *
 * Once the HTML is in, the app's enhancements run on it in their order;
 * before the HTML is replaced, and when the view goes, they are undone in
 * the reverse order (reading/enhancement.ts). A writer's tick task items
 * through it, one of the page's at a time (the page tree store's
 * oneToggle), and refused tells the page what was refused, undefined as a
 * toggle starts (M5/P6 design 3.5). A task item's checkbox that had the
 * focus as the HTML is replaced has it back in the new HTML, without a
 * scroll, while the box at its position is the same item's (its text, and
 * its state, or the state the view's own toggle asked for): a tick does
 * not move it, a write that moved the items does.
 *
 * An enhancement goes to another address through the router: to a page,
 * whose heading takes the focus; or to an anchor. An address with an
 * anchor has the view go to the element it names, once the HTML is in,
 * once for each time the app goes to the address: the element shows and
 * takes the focus (M6/P3 design 6.7). The page keeps the last navigation
 * its view took in (anchored), as the view goes while the page is edited:
 * coming back from editing goes nowhere. When the anchor names no element
 * as the page opens (its first navigation, by a load or from another
 * page), the view is read again first if it came from the cache, as it may
 * be older than the link; then, if the focus is nowhere (what had it went
 * with the page before), unanchored gives it the page's heading. A link of
 * the page to no element leaves the focus where it is, and the page. An
 * element with an id that had the focus as the HTML is replaced, the
 * anchor's among them, has it back in the new HTML, shown again if it
 * showed and no longer does: a view read again stays where it is.
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
  const enhancements = useContext(Enhancements);
  const navigate = useNavigate();
  const location = useLocation();
  // The element with an id focused as the HTML was replaced, and whether it showed.
  const focusedTarget = useRef<{ id: string; shown: boolean } | undefined>(undefined);
  // The navigation whose anchor named no element of the view, which is read again for it (the view, from the
  // cache, may be older than the link), and the one whose read for it is in.
  const rereading = useRef<string | undefined>(undefined);
  const [reread, setReread] = useState<string | undefined>(undefined);
  const { data, error, mutate } = useSWR(["page-view", notebook.id, page.id], () => pages.view(page.id));
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
  useEffect(() => {
    latestRefused.current = refused;
    latestUnanchored.current = unanchored;
  });
  const html = data?.html;
  const revision = data?.revision;
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
  }, [html, revision, enhancements, slug, notebookId, role, page.id, mutate, pages, navigate]);
  useLayoutEffect(() => {
    const container = article.current;
    const navigation = location.key + location.hash;
    if (container === null || html === undefined || anchored.current === navigation) {
      return;
    }
    const anchor = location.hash.slice(1);
    const target = anchor === "" ? undefined : named(container, anchor);
    // The page's first navigation: it opened, by a load or from another page.
    const opened = anchored.current === undefined;
    if (anchor !== "" && target === undefined && opened && cached.current && reread !== navigation) {
      if (rereading.current !== navigation) {
        rereading.current = navigation;
        void mutate().finally(() => setReread(navigation));
      }
      return;
    }
    anchored.current = navigation;
    if (target !== undefined) {
      focusOn(target, {});
    } else if (anchor !== "" && opened && document.activeElement === document.body) {
      latestUnanchored.current();
    }
  }, [anchored, html, location.hash, location.key, mutate, reread]);
  if (data === undefined) {
    return <NotLoaded error={error} retry={() => void mutate()} />;
  }
  // Named by the page: it can get the focus to scroll a wide content (reading/scroll-focus.ts).
  return <article ref={article} aria-label={page.name} className="nw-reading min-w-0" />;
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

/** focusOn gives element the focus, focusable as an anchor's target is, scrolled into view as show says, if it does. */
function focusOn(element: HTMLElement, show: ScrollIntoViewOptions | undefined) {
  if (!element.hasAttribute("tabindex")) {
    element.setAttribute("tabindex", "-1");
  }
  if (show !== undefined) {
    element.scrollIntoView(show);
  }
  element.focus({ preventScroll: true });
}
