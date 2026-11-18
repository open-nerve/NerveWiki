import { useLayoutEffect, useRef, type RefObject } from "react";

import { enhance, type Enhancement, type ReadingContext } from "../../reading/enhancement";
import {
  addressesOnly,
  findByKey,
  focusKeyOf,
  patchBlocks,
  resign,
  template,
  type FocusKey,
  type Placed,
} from "../../reading/patch";
import { scrolling } from "../../reading/scroll-regions";
import { taskText } from "../../reading/task-toggle";
import { unfold } from "../../reading/unfold";
import { shows } from "./view-anchors";

/** ServerView is the server's view of a page that a reading view writes: its HTML, its revision, when its addresses expire. */
export type ServerView = { html: string; revision: number; assetsExpire: string | null };

/** Toggled is the view's own toggle under way: its item's position and the state asked for (M5/P6 design 3.5). */
export type Toggled = { task: string; checked: boolean };

/**
 * ServerHTMLContext is what the enhancements run with, but for what the view brings (its revision, its addresses'
 * expiry): the enhancements, in their order; the view's own toggle, which a view showing it ends; and the notebook's
 * mode, by which a link to an LLM Wiki's index or log leads (the context's leads), which the enhancements read as they
 * run.
 */
export type ServerHTMLContext = Omit<ReadingContext, "revision" | "assetsExpire"> & {
  enhancements: readonly Enhancement[];
  toggled?: RefObject<Toggled | undefined>;
  mode?: string;
};

/**
 * Shown is the server's HTML in the article (M12/P2/S4, 4.4.2): the article, the HTML and the template it parses
 * to, how its enhancements are undone, and what they ran with: the same, or in another language or mode (spoken).
 */
type Shown = {
  container: HTMLElement;
  html: string;
  template: HTMLTemplateElement;
  undo: () => void;
  ran: readonly unknown[];
  spoken: readonly unknown[];
};

/**
 * useServerHTML writes view, the server's HTML of a page, into article, and runs the enhancements on it (M4/P5
 * design 3.8): as the view changes, the enhancements are undone in the reverse order, the HTML changed, and they run
 * again; as it goes, they are undone. The context's callbacks are its latest's, and so are the view's revision and
 * its addresses' expiry. Whether the page can be written changes no HTML: the enhancements hear it (writes,
 * onWritesChange; M12/P2/S4, 48).
 *
 * The same HTML read again, at another revision maybe (a save of blank lines), is left as it is (U2). One whose
 * attachments' addresses alone differ, signed anew for the next hour, has them given to its elements in place (U3,
 * resign): no node taken out or put in, no enhancement run again (M12/P2/S4, 17.1). Another is written where it
 * changed the view, block by block (U4, patchBlocks): the blocks it left are the same nodes, the selection in them,
 * the callouts opened, a video in fullscreen (M12/P2/S4, 3, 8.2, 17.8). The language or the notebook's mode
 * changing leaves the HTML as it is: the enhancements run again on it (U5; M12/P2/S4, 11.6, 48).
 *
 * The focus stays (M12/P2/S4, 4.4.3): what had it as the view changed has it back, if it is there still; else, a
 * task item's checkbox at its position while it is the same item's (its text, and its state, or the state the view's
 * own toggle asked for), none if not: a tick does not move it, a write that moved the items does; else the element
 * of its key in the block it was in, as the patch placed it (focusKeyOf: a heading of its text, a link of its
 * address and text, a callout's summary, a media, an element of its id; a scroll region, a diagram's drawn anew,
 * which of its block's), not one of its key elsewhere, which the view would scroll to; else the block where it was,
 * without a scroll or a callout opened. Shown as it is given the
 * focus, a folded callout opens; one that showed and shows no more is scrolled to: a view read again stays where it
 * is.
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
  const shown = useRef<Shown | undefined>(undefined);
  const html = view?.html;
  const { enhancements, workspace, notebook, page, t, locale, mode } = context;
  useLayoutEffect(() => {
    const container = article.current;
    const was = shown.current;
    const ran = [enhancements, workspace, notebook, page];
    const spoken = [t, locale, mode];
    // The view shown still, its enhancements running as they ran but maybe in another language or mode.
    const showing = was !== undefined && container === was.container && html !== undefined && sameRun(was.ran, ran);
    const same = showing && sameRun(was.spoken, spoken);
    let read: HTMLTemplateElement | undefined;
    if (showing && html !== was.html) {
      read = template(html);
      if (same && addressesOnly(was.template, read) && resign(container, was.template, read)) {
        shown.current = { ...was, html, template: read };
        return;
      }
    } else if (same) {
      return;
    }
    const focus = was === undefined ? undefined : noted(was.container);
    if (was !== undefined) {
      shown.current = undefined;
      was.undo();
    }
    if (container === null || html === undefined) {
      return;
    }
    // A change of the view is written where it changed it (U4): read is the view read again. The same HTML in
    // another language or mode is left as it is (U5): the enhancements run again on it. Else, or its blocks not the
    // view's, it is written whole.
    let placed: Placed | undefined;
    if (showing && read === undefined) {
      placed = (at) => at;
    } else if (showing && read !== undefined) {
      placed = patchBlocks(container, was.template, read);
    }
    if (placed === undefined) {
      write(container, html);
      placed = (at) => clamped(at, container);
    }
    read ??= was !== undefined && html === was.html ? was.template : template(html);
    // The view as the HTML was written: the context reads the latest view's, while there is one.
    const written = latest.current.view ?? { revision: 0, assetsExpire: null };
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
    shown.current = { container, html, template: read, undo, ran, spoken };
    const toggled = now().toggled;
    const asked = toggled?.current;
    const box = (task: string) => container.querySelector<HTMLElement>(`input[data-task="${CSS.escape(task)}"]`);
    if (toggled !== undefined && asked !== undefined && box(asked.task)?.hasAttribute("checked") === asked.checked) {
      toggled.current = undefined;
    }
    if (focus !== undefined) {
      giveBack(container, focus, placed, asked);
    }
  }, [article, html, enhancements, workspace, notebook, page, t, locale, mode]);
  // Gone, the view's enhancements are undone (U8).
  useLayoutEffect(
    () => () => {
      const was = shown.current;
      shown.current = undefined;
      was?.undo();
    },
    []
  );
}

/** sameRun tells whether the enhancements ran with what they would run with now: each the same. */
function sameRun(ran: readonly unknown[], now: readonly unknown[]): boolean {
  return ran.length === now.length && ran.every((value, i) => Object.is(value, now[i]));
}

/** write puts html in container, in place of all it had. */
function write(container: HTMLElement, html: string) {
  container.innerHTML = html;
}

/** clamped is the place at in container written whole: the same, within it. */
function clamped(at: number, container: HTMLElement): number | undefined {
  const count = container.childNodes.length;
  return count === 0 ? undefined : Math.min(Math.max(at, 0), count - 1);
}

/**
 * Noted is what had the focus as the view changed (M12/P2/S4, 4.4.3): the element, whether it showed, its key, the
 * place of the top-level node it is in (none for the article itself); a task item's checkbox its position, text and
 * state.
 */
type Noted = {
  element: HTMLElement;
  shown: boolean;
  key: FocusKey | undefined;
  block: number | undefined;
  region: number | undefined;
  task: { task: string; text: string; checked: boolean } | undefined;
};

/**
 * noted is what has the focus in container, before its enhancements are undone. Undone, a checkbox is disabled
 * again, which HTML's focus fixup takes the focus from: Chromium at the next rendering, an engine that applies the
 * rule at once before the new HTML is in. The top-level node is the same once they are: they change one for one.
 */
function noted(container: HTMLElement): Noted | undefined {
  const active = document.activeElement;
  if (!(active instanceof HTMLElement) || !container.contains(active)) {
    return undefined;
  }
  const block = [...container.childNodes].findIndex((node) => node.contains(active));
  const top = container.childNodes[block];
  // Its key's place is counted in the block it is in, the article itself's in the article.
  const key = focusKeyOf(top instanceof Element ? top : container, active);
  return {
    element: active,
    shown: shows(active),
    key,
    block: block < 0 ? undefined : block,
    // A scroll region of no other key is which of its block's: a diagram's wrapper is drawn anew.
    region: key === undefined && block >= 0 ? regions(container.childNodes[block]).indexOf(active) : undefined,
    task:
      active instanceof HTMLInputElement && active.dataset.task !== undefined
        ? { task: active.dataset.task, text: taskText(active), checked: active.hasAttribute("checked") }
        : undefined,
  };
}

/**
 * giveBack gives the focus back in container, the view changed, its blocks placed so (M12/P2/S4, 4.4.3): to what
 * had it, if it is there still; to a task item's checkbox at its position, the same item's (or none); to the element
 * of its key in the block it maps to; else to that block, or the article.
 */
function giveBack(container: HTMLElement, focus: Noted, placed: Placed, asked: Toggled | undefined) {
  const { element, task } = focus;
  if (container.contains(element)) {
    give(element, focus.shown);
    return;
  }
  if (task !== undefined) {
    const box = container.querySelector<HTMLElement>(`input[data-task="${CSS.escape(task.task)}"]`);
    // The same item: its text, and its state, or the one the view's own toggle asked for.
    const state = box?.hasAttribute("checked");
    if (
      box &&
      taskText(box) === task.text &&
      (state === task.checked || (asked?.task === task.task && state === asked.checked))
    ) {
      unfold(box);
      box.focus({ preventScroll: true });
    }
    return;
  }
  const at = focus.block === undefined ? undefined : placed(focus.block);
  const block = at === undefined ? undefined : container.childNodes[at];
  // In the block it maps to (or the article, for the article's own): one of its key elsewhere is not it.
  const within = focus.block === undefined ? container : block;
  const again =
    (focus.key && within instanceof Element && findByKey(within, focus.key)) ||
    (focus.region !== undefined && focus.region >= 0 ? regions(block)[focus.region] : undefined);
  if (again) {
    give(again, focus.shown);
    return;
  }
  const place = block instanceof HTMLElement ? block : container;
  // Not opened nor scrolled to: the focus is where it was, the reader's view as it is.
  if (!focusable(place)) {
    place.setAttribute("tabindex", "-1");
  }
  place.focus({ preventScroll: true });
}

/** regions are the scroll regions of a top-level node, it among them (scroll-regions.ts), in order. */
function regions(block: Node | undefined): HTMLElement[] {
  if (!(block instanceof HTMLElement)) {
    return [];
  }
  return [block, ...block.querySelectorAll<HTMLElement>(scrolling)].filter((element) => element.matches(scrolling));
}

/** What takes the focus of itself. */
const natively =
  "a[href], summary, button, input, select, textarea, audio[controls], video[controls], iframe, [tabindex]";

/** focusable tells whether element takes the focus of itself. */
function focusable(element: HTMLElement): boolean {
  return element.matches(natively) || element.isContentEditable;
}

/**
 * give gives element the focus, without a scroll but for one that showed and shows no more: scrolled into view, a
 * scroll of a wide content's would be reset too. A folded callout it is in opens, not one whose summary it is.
 * Focusable as an anchor's target is.
 */
function give(element: HTMLElement, showed: boolean) {
  unfold(element.localName === "summary" ? (element.parentElement ?? element) : element);
  if (!focusable(element)) {
    element.setAttribute("tabindex", "-1");
  }
  if (showed && !shows(element)) {
    element.scrollIntoView({ block: "nearest" });
  }
  element.focus({ preventScroll: true });
}
