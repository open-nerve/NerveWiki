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
 * useArrivalFocus is the ref of a page's main heading (tabIndex -1), which
 * takes the focus as the page shows when the page was arrived at, and does
 * not take it otherwise.
 */
export function useArrivalFocus<T extends HTMLElement>(): RefObject<T | null> {
  const ref = useRef<T>(null);
  const arrival = isArrival(useLocation().state);
  useEffect(() => {
    if (arrival) {
      ref.current?.focus();
    }
  }, [arrival]);
  return ref;
}
