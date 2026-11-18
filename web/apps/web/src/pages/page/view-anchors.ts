import { useLayoutEffect, useRef, type RefObject } from "react";
import { useLocation, useNavigationType } from "react-router";

import { unfold } from "../../reading/unfold";
import { watchReader } from "./readers-input";

/**
 * useViewAnchors has the reading view go to the element an address's
 * anchor names (M6/P3 design 6.7), once its HTML is in, once for each
 * time the app goes to the address: the element shows and takes the
 * focus. The page keeps the last navigation its view took in (anchored),
 * as the view goes while the page is edited: coming back from editing
 * goes nowhere. When the anchor names no element as the page opens (its
 * first navigation, by a load or from another page), unanchored gives the
 * page's heading the focus if it is nowhere (what had it went with the
 * page before); and a view from the cache (cached), which may be older
 * than the link, is read again (reload): the element it brings shows and
 * takes the focus, if the reader has done nothing since (scrolled,
 * clicked, touched, pressed a key: the browser's own scrolling is none of
 * these, nor are an assistive technology's moves) and the focus is where
 * it was; a read after that one moves nothing. A link of the page to no
 * element leaves the focus where it is, and the page. html is the view's
 * HTML: the view read again for the anchor has it anew (M12/P2/S4, 14d).
 * Back or forward to an address with an anchor, the browser puts back the
 * scroll its history entry had after the view went to the element: at
 * the next frame, the window scrolled since, the element having the focus
 * still, the reader having done nothing, and out of view, it is shown
 * again (M12/P2/S4, 8.1).
 */
export function useViewAnchors({
  article,
  html,
  anchored,
  unanchored,
  cached,
  reload,
}: {
  article: RefObject<HTMLElement | null>;
  html: string | undefined;
  // The last navigation the page's view took in, its anchor gone to: the location's key and fragment (one key may
  // have several: an address typed in).
  anchored: RefObject<string | undefined>;
  /** unanchored is the page's latest: its heading takes the focus, if it is nowhere. */
  unanchored: RefObject<() => void>;
  /** Whether the view came from the cache: older, maybe, than the address. */
  cached: RefObject<boolean>;
  /** reload reads the view again, and settles once it is read. */
  reload: () => Promise<unknown>;
}): void {
  const location = useLocation();
  const popped = useNavigationType() === "POP";
  // While the view from the cache is read again for the anchor the page opened at, which named no element of it:
  // what had the focus then, and how the wait ends.
  const awaited = useRef<{ focus: Element | null; end: () => void } | undefined>(undefined);
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
    const opened = takeIn(anchored, navigation);
    awaited.current?.end();
    if (target !== undefined) {
      focusOn(target, {});
      if (popped) {
        showAgain(target);
      }
      return;
    }
    if (anchor === "" || !opened) {
      return;
    }
    if (document.activeElement === document.body) {
      unanchored.current();
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
      void reload().finally(end);
    }
  }, [anchored, article, cached, html, location.hash, location.key, popped, reload, unanchored]);
}

/**
 * showAgain shows target at the next frame if the window was scrolled since the view went there, it has the focus
 * still, the reader having done nothing, and it is out of view: the browser put back the scroll of the history entry
 * gone back or forward to (M12/P2/S4, 8.1; in the three engines, after the view's scroll and before the next frame).
 */
function showAgain(target: HTMLElement) {
  const reader = watchReader();
  const at = [window.scrollX, window.scrollY];
  requestAnimationFrame(() => {
    reader.end();
    const scrolled = window.scrollX !== at[0] || window.scrollY !== at[1];
    if (scrolled && !reader.acted() && document.activeElement === target && !shows(target)) {
      target.scrollIntoView({});
    }
  });
}

/** takeIn has the page's view take navigation in, and tells whether it is the page's first: it opened. */
function takeIn(anchored: RefObject<string | undefined>, navigation: string): boolean {
  const opened = anchored.current === undefined;
  anchored.current = navigation;
  return opened;
}

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
export function shows(element: HTMLElement): boolean {
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
