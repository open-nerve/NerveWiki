import { useSyncExternalStore } from "react";

/** Below md, Tailwind's max-md: (width < 48rem). */
const query = "(width < 48rem)";
/** From xl up, Tailwind's xl: (width >= 80rem): the page's right column beside its content (M13/P4/S1). */
const besideQuery = "(width >= 80rem)";

/** subscriber listens to the window's width crossing of's bound. */
function subscriber(of: string): (changed: () => void) => () => void {
  return (changed) => {
    if (typeof window.matchMedia !== "function") {
      return () => {};
    }
    const list = window.matchMedia(of);
    list.addEventListener("change", changed);
    return () => list.removeEventListener("change", changed);
  };
}

const subscribe = subscriber(query);
const subscribeBeside = subscriber(besideQuery);

function narrow(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia(query).matches;
}

function beside(): boolean {
  return typeof window.matchMedia !== "function" || window.matchMedia(besideQuery).matches;
}

/**
 * useNarrow tells whether the window is narrower than md, a phone's width, the component rendering again as that
 * changes: the account's menu has the appearance in it there, no submenu (M13/P2/S1, decision 801), and the left
 * column is a drawer (M13/P2/S3). Without matchMedia, as in jsdom, it is not.
 */
export function useNarrow(): boolean {
  return useSyncExternalStore(subscribe, narrow);
}

/**
 * useBeside tells whether the window is xl or wider, the page's right column beside its content, held and the
 * window's height, which its toggle folds away (M13/P4/S1, decisions 859, 886), the component rendering again as that
 * changes. Without matchMedia, as in jsdom, it is: the right column's tests run beside the page.
 */
export function useBeside(): boolean {
  return useSyncExternalStore(subscribeBeside, beside);
}
