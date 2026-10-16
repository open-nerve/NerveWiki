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

/**
 * The navigations whose heading has been offered the arrival focus, or has
 * mounted without it: React Router's location objects, one per navigation
 * (its Router memoizes the location on the path, search, hash, state, key
 * and the navigation's type), the same across the navigation's re-renders,
 * new for a push, a replace, Back and Forward alike.
 */
const offered = new WeakSet<object>();

/**
 * takeArrivalFocus gives heading the arrival focus of the navigation it
 * mounts for. The first heading of a page that mounts for a navigation
 * takes it wherever it is: the search box's submit arrives at the results
 * with the focus still in the box, and Back or Forward to a page arrived
 * at takes it from the tree's link or the box, as a new navigation. One
 * that mounts later for the same navigation takes it only from no one (the
 * body), as the page shown turns into another under the reader: deleted
 * elsewhere, its shell gives way to "Page not found"; restored elsewhere,
 * its shell comes back. A reader working elsewhere meanwhile, in the
 * search box say, keeps the focus and the keys they type (v0.1 design
 * 13.2, item 26; M8 closeout K1-1, K2-1, K2-2).
 */
function takeArrivalFocus(navigation: object, heading: HTMLElement | null): void {
  const first = !offered.has(navigation);
  offered.add(navigation);
  const at = document.activeElement;
  if (first || at === null || at === document.body) {
    heading?.focus();
  }
}

/**
 * useArrivalFocus is the ref of a page's main heading (tabIndex -1), which
 * takes the focus as the page shows when the page was arrived at, or when
 * also says the page's own state is an arrival too, and does not take it
 * otherwise (takeArrivalFocus). A heading that mounts for a navigation
 * that is no arrival still counts as its first: a page opened for an
 * attachment (Located), the attachment's row taking the focus, then
 * deleted elsewhere, gives way to "Page not found" as a later mount, which
 * takes the focus only from no one (M8 closeout K2-1).
 */
export function useArrivalFocus<T extends HTMLElement>(also = false): RefObject<T | null> {
  const ref = useRef<T>(null);
  const location = useLocation();
  const arrival = also || isArrival(location.state);
  const entry = useRef<object>(location);
  useEffect(() => {
    entry.current = location;
  });
  useEffect(() => {
    if (arrival) {
      takeArrivalFocus(entry.current, ref.current);
    } else {
      offered.add(entry.current);
    }
  }, [arrival]);
  return ref;
}
