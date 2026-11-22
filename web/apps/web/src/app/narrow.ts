import { useSyncExternalStore } from "react";

/** Below md, Tailwind's max-md: (width < 48rem). */
const query = "(width < 48rem)";

function subscribe(changed: () => void): () => void {
  if (typeof window.matchMedia !== "function") {
    return () => {};
  }
  const list = window.matchMedia(query);
  list.addEventListener("change", changed);
  return () => list.removeEventListener("change", changed);
}

function narrow(): boolean {
  return typeof window.matchMedia === "function" && window.matchMedia(query).matches;
}

/**
 * useNarrow tells whether the window is narrower than md, a phone's width, the component rendering again as that
 * changes: the account's menu has the appearance in it there, no submenu (M13/P2/S1, decision 801), and the left
 * column is a drawer (M13/P2/S3). Without matchMedia, as in jsdom, it is not.
 */
export function useNarrow(): boolean {
  return useSyncExternalStore(subscribe, narrow);
}
