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
 * they run again; as it goes, they are undone. The context's callbacks are its latest's.
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
  const latest = useRef(context);
  useLayoutEffect(() => {
    latest.current = context;
  });
  // The task item focused as the HTML was replaced: its position, its text and its state.
  const focusedTask = useRef<{ task: string; text: string; checked: boolean } | undefined>(undefined);
  // The element with an id focused as the HTML was replaced, and whether it showed.
  const focusedTarget = useRef<{ id: string; shown: boolean } | undefined>(undefined);
  const html = view?.html;
  const revision = view?.revision;
  const assetsExpire = view?.assetsExpire ?? null;
  const { enhancements, workspace, notebook, page, role, t, locale } = context;
  const toggles = context.toggleTask !== undefined;
  useLayoutEffect(() => {
    const container = article.current;
    if (container === null || html === undefined || revision === undefined) {
      return undefined;
    }
    write(container, html);
    const undo = enhance(enhancements, container, {
      workspace,
      notebook,
      page,
      revision,
      role,
      t,
      locale,
      theme: () => latest.current.theme(),
      onThemeChange: (listener) => latest.current.onThemeChange(listener),
      reload: () => latest.current.reload(),
      navigate: (to) => latest.current.navigate(to),
      toggleTask: toggles
        ? (offset, checked) => latest.current.toggleTask?.(offset, checked) ?? Promise.resolve()
        : undefined,
      report: (error) => latest.current.report(error),
      unresolved: (link) => latest.current.unresolved(link),
      leads: (target) => latest.current.leads?.(target) ?? false,
      assetsExpire,
      assetAddress: (id) => latest.current.assetAddress(id),
    });
    const focused = focusedTask.current;
    const toggled = latest.current.toggled;
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
  }, [article, html, revision, assetsExpire, enhancements, workspace, notebook, page, role, toggles, t, locale]);
}

/** write puts html in container, in place of all it had. */
function write(container: HTMLElement, html: string) {
  container.innerHTML = html;
}
