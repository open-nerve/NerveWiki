import { useLayoutEffect, useRef, type RefObject } from "react";

import { enhance, type Enhancement, type ReadingContext } from "../../reading/enhancement";
import { taskText } from "../../reading/task-toggle";
import { unfold } from "../../reading/unfold";
import { byId, focusOn, shows } from "./view-anchors";

/** ServerView is the server's view of a page that a reading view writes: its HTML, its revision, when its addresses expire. */
export type ServerView = { html: string; revision: number; assetsExpire: string | null };

/** Toggled is the view's own toggle under way: its item's position and the state asked for (M5/P6 design 3.5). */
export type Toggled = { task: string; checked: boolean };

/**
 * ServerHTMLContext is what the enhancements run with, but for what the view brings (its revision, its addresses'
 * expiry): the enhancements, in their order; and the view's own toggle, which a view showing it ends.
 */
export type ServerHTMLContext = Omit<ReadingContext, "revision" | "assetsExpire"> & {
  enhancements: readonly Enhancement[];
  toggled?: RefObject<Toggled | undefined>;
};

/**
 * useServerHTML writes view, the server's HTML of a page, into article, and runs the enhancements on it (M4/P5
 * design 3.8): as the view changes, the enhancements are undone in the reverse order, the HTML written again, and
 * they run again; as it goes, they are undone. The context's callbacks are its latest's, and so are the view's
 * revision and its addresses' expiry. Whether the page can be written changes no HTML: the enhancements hear it
 * (writes, onWritesChange; M12/P2/S4, 48).
 *
 * A task item's checkbox that had the focus as the HTML is replaced has it back in the new HTML, without a scroll,
 * while the box at its position is the same item's (its text, and its state, or the state the view's own toggle
 * asked for): a tick does not move it, a write that moved the items does. A folded callout it is in opens again, as
 * the reader had it. An element with an id that had the focus, the anchor's among them, has it back in the new
 * HTML, shown again if it showed and no longer does: a view read again stays where it is (M12/P2/S4, 14d).
 */
export function useServerHTML(
  article: RefObject<HTMLElement | null>,
  view: ServerView | undefined,
  context: ServerHTMLContext
): void {
  const latest = useRef({ context, view });
  useLayoutEffect(() => {
    latest.current = { context, view };
  });
  // The task item focused as the HTML was replaced: its position, its text and its state.
  const focusedTask = useRef<{ task: string; text: string; checked: boolean } | undefined>(undefined);
  // The element with an id focused as the HTML was replaced, and whether it showed.
  const focusedTarget = useRef<{ id: string; shown: boolean } | undefined>(undefined);
  const html = view?.html;
  const revision = view?.revision;
  const { enhancements, workspace, notebook, page, t, locale } = context;
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined || revision === undefined) {
      return undefined;
    }
    write(container, html);
    // The view as the HTML was written: the context reads the latest view's, while there is one.
    const written = latest.current.view ?? { revision, assetsExpire: null };
    const now = () => latest.current.context;
    const { toggleTask, writes, onWritesChange } = now();
    const undo = enhance(enhancements, container, {
      workspace,
      notebook,
      page,
      // The view's as it is: the same HTML read again may come at another revision, its addresses signed anew.
      get revision() {
        return (latest.current.view ?? written).revision;
      },
      get assetsExpire() {
        return (latest.current.view ?? written).assetsExpire;
      },
      t,
      locale,
      theme: () => now().theme(),
      onThemeChange: (listener) => now().onThemeChange(listener),
      reload: () => now().reload(),
      navigate: (to) => now().navigate(to),
      toggleTask:
        toggleTask === undefined
          ? undefined
          : (offset, checked) => now().toggleTask?.(offset, checked) ?? Promise.resolve(),
      writes: writes === undefined ? undefined : () => now().writes?.() ?? true,
      onWritesChange:
        onWritesChange === undefined ? undefined : (listener) => now().onWritesChange?.(listener) ?? (() => undefined),
      report: (error) => now().report(error),
      unresolved: (link) => now().unresolved(link),
      leads: (target) => now().leads?.(target) ?? false,
      assetAddress: (id) => now().assetAddress(id),
    });
    const focused = focusedTask.current;
    const toggled = now().toggled;
    const asked = toggled?.current;
    focusedTask.current = undefined;
    const at = (task: string) => container.querySelector<HTMLElement>(`input[data-task="${CSS.escape(task)}"]`);
    if (toggled !== undefined && asked !== undefined && at(asked.task)?.hasAttribute("checked") === asked.checked) {
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
  }, [article, html, revision, enhancements, workspace, notebook, page, t, locale]);
}

/** write puts html in container, in place of all it had. */
function write(container: HTMLElement, html: string) {
  container.innerHTML = html;
}
