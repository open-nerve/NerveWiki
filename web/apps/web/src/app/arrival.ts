import { useEffect, useRef, type RefObject } from "react";
import { useLocation } from "react-router";

/**
 * arrived is the route state of a page reached because the one before went
 * away with the button that was pressed there (M3/P4 design 3.5): a
 * workspace or a notebook deleted or left, an invitation accepted, a
 * notebook created. The page's main heading takes the focus as it shows,
 * which would otherwise fall to the body.
 */
export const arrived = { arrived: true } as const;

function isArrival(state: unknown): boolean {
  return typeof state === "object" && state !== null && "arrived" in state && state.arrived === true;
}

/** The history entries (their location's key) a heading has taken the arrival focus on, or been offered it. */
const offered = new Set<string>();

/**
 * takeArrivalFocus gives heading the arrival focus of the history entry
 * key. The first heading that mounts for the entry takes it wherever it
 * is: the search box's submit arrives at the results with the focus still
 * in the box. One that mounts later for the same entry takes it only from
 * no one (the body), as the page shown turns into another under the
 * reader: deleted elsewhere, its shell gives way to "Page not found";
 * restored elsewhere, its shell comes back. A reader working elsewhere
 * meanwhile, in the search box say, keeps the focus and the keys they
 * type (v0.1 design 13.2, item 26; M8 closeout K1-1). Gone back to, an
 * entry's heading mounts later too: the page left went with the focus it
 * had, so no one has it, and the heading takes it.
 */
function takeArrivalFocus(key: string, heading: HTMLElement | null): void {
  const first = !offered.has(key);
  offered.add(key);
  const at = document.activeElement;
  if (first || at === null || at === document.body) {
    heading?.focus();
  }
}

/**
 * useArrivalFocus is the ref of a page's main heading (tabIndex -1), which
 * takes the focus as the page shows when the page was arrived at, or when
 * also says the page's own state is an arrival too, and does not take it
 * otherwise (takeArrivalFocus).
 */
export function useArrivalFocus<T extends HTMLElement>(also = false): RefObject<T | null> {
  const ref = useRef<T>(null);
  const location = useLocation();
  const arrival = also || isArrival(location.state);
  const entry = useRef(location.key);
  useEffect(() => {
    entry.current = location.key;
  });
  useEffect(() => {
    if (arrival) {
      takeArrivalFocus(entry.current, ref.current);
    }
  }, [arrival]);
  return ref;
}
