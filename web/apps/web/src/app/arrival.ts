import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { useLocation, useNavigate, useNavigationType, type Location, type NavigationType } from "react-router";

import { watchReader } from "../pages/page/readers-input";

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
 * opened is the route state of a page the notebook's tree opened (M12/P2/S1): no arrival, the focus staying on the
 * tree's link, but an entry Back and Forward come to after the page left took the focus with it, its title taking
 * it then from no one (useArrivalFocus).
 */
export const opened = { opened: true } as const;

function isOpened(state: unknown): boolean {
  return typeof state === "object" && state !== null && "opened" in state && state.opened === true;
}

/**
 * isLocated tells whether state is Located's, a page gone to for one of its attachments, as attachment-row.tsx's
 * locatedOf reads it: not taken from there, which is in a chunk loaded later, with the menu's parts.
 */
function isLocated(state: unknown): boolean {
  return typeof state === "object" && state !== null && "attachment" in state && typeof state.attachment === "string";
}

/**
 * headingTakes tells whether the page a navigation of type goes to, its route state state, takes the focus itself:
 * arrived at, Back or Forward to a page the tree opened (as useArrivalFocus's arrival and fromNoOne), or gone to for an
 * attachment (Located), whose row, or the title saying it is not found, takes it whatever the navigation. The drawer
 * of a phone's width, closed by the navigation, gives its menu the focus otherwise (M13/P2/S3, decision 790).
 */
export function headingTakes(state: unknown, type: NavigationType): boolean {
  return isArrival(state) || isLocated(state) || (type === "POP" && isOpened(state));
}

type Reader = ReturnType<typeof watchReader>;

/**
 * What the reader did since each arrival's navigation committed, by its location (useArrivalWatch), until the next
 * navigation.
 */
const watching = new WeakMap<object, Reader>();

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
 * 13.2, item 26; M8 closeout K1-1, K2-1, K2-2). The first heading of an
 * arrival that mounts once the reader did something since its navigation
 * committed (useArrivalWatch) counts as a later one: a title that shows
 * late, the notebook's tree still read, does not take the box the reader
 * types in (M12/P2/S1, 28.4).
 */
function takeArrivalFocus(navigation: object, heading: HTMLElement | null): void {
  const first = !offered.has(navigation) && watching.get(navigation)?.acted() !== true;
  offered.add(navigation);
  const at = document.activeElement;
  if (first || at === null || at === document.body) {
    heading?.focus();
    // WebKit does not scroll to a focused heading near the top of a page left scrolled down.
    if (heading !== null && heading.getBoundingClientRect().bottom < 0) {
      heading.scrollIntoView({ block: "center" });
    }
  }
}

/**
 * useArrivalWatch watches what the reader does from each arrival's
 * navigation as it commits (watchReader) until the next one: Layout, the
 * root route's component, calls it once, its layout effect running
 * before the headings' effects of the same commit. A Located navigation
 * is not watched: an attachment's row watches the reader itself
 * (useLocated). A replace of the entry's state alone, its path, search
 * and hash the same, is the same navigation: the watch goes on, not
 * started again, and a heading already offered the focus stays offered
 * (M12/P2/S1, 28.4): a route's notice cleared by such a replace leaves a
 * later "Page not found" a later mount. A replace of the same path that
 * changes its query or hash, a filter's, is a new navigation, watched
 * anew; the page stays, its heading not mounting again, and it stays
 * offered too: a later "Page not found" takes the focus from no one only
 * (M12/P2/S1, review B's M1).
 */
export function useArrivalWatch(): void {
  const location = useLocation();
  const type = useNavigationType();
  const last = useRef<{ location: Location; reader: Reader | undefined }>(undefined);
  useLayoutEffect(() => {
    const before = last.current;
    if (before?.location === location) {
      return;
    }
    const same =
      before !== undefined &&
      type === "REPLACE" &&
      before.location.pathname === location.pathname &&
      before.location.search === location.search &&
      before.location.hash === location.hash;
    if (same) {
      if (before.reader !== undefined) {
        watching.set(location, before.reader);
      }
      if (offered.has(before.location)) {
        offered.add(location);
      }
      last.current = { location, reader: before.reader };
      return;
    }
    if (
      before !== undefined &&
      type === "REPLACE" &&
      before.location.pathname === location.pathname &&
      offered.has(before.location)
    ) {
      offered.add(location);
    }
    before?.reader?.end();
    const reader = isArrival(location.state) ? watchReader() : undefined;
    if (reader !== undefined) {
      watching.set(location, reader);
    }
    last.current = { location, reader };
  }, [location, type]);
  // Unmounted (StrictMode's too, which mounts it again with the same location), the watch ends and starts anew.
  useLayoutEffect(
    () => () => {
      last.current?.reader?.end();
      last.current = undefined;
    },
    []
  );
}

/**
 * useRouteNotice is what the route's state says as the page mounts, of
 * (a page's notice: a version restored, an item restored or a job started
 * in the trash, a node of an item gone to its preview), for this mount to
 * say; once it says something, the entry's state is replaced without keys,
 * so that a reload, or Back and Forward to the entry, says it no more
 * (M5-polish 21.3, 22.2). The arrival stays in the state, and the replace
 * of the state alone is the same navigation (useArrivalWatch): the
 * heading's focus is not offered again, and a later "Page not found" takes
 * it from no one.
 */
export function useRouteNotice<T>(of: (state: unknown) => T | undefined, keys: readonly string[]): T | undefined {
  const location = useLocation();
  const navigate = useNavigate();
  const [notice] = useState(() => of(location.state));
  const at = useRef({ location, keys });
  // Once: StrictMode's second mount finds the entry replaced already.
  const taken = useRef(false);
  useEffect(() => {
    if (notice === undefined || taken.current) {
      return;
    }
    taken.current = true;
    const { location: entry, keys: taking } = at.current;
    const state = typeof entry.state === "object" && entry.state !== null ? entry.state : {};
    void navigate(
      { pathname: entry.pathname, search: entry.search, hash: entry.hash },
      { replace: true, state: Object.fromEntries(Object.entries(state).filter(([key]) => !taking.includes(key))) }
    );
  }, [notice, navigate]);
  return notice;
}

/**
 * useArrivalFocus is the ref of a page's main heading (tabIndex -1), which
 * takes the focus as the page shows when the page was arrived at, or when
 * also says the page's own state is an arrival too, and does not take it
 * otherwise (takeArrivalFocus). A heading that mounts for a navigation
 * that is no arrival still counts as its first: a page opened for an
 * attachment (Located), the attachment's row taking the focus, then
 * deleted elsewhere, gives way to "Page not found" as a later mount, which
 * takes the focus only from no one (M8 closeout K2-1). Of no arrival, it
 * takes the focus from no one when Back or Forward come to a page the
 * tree opened (opened), or when replacing says it stands for the page
 * this navigation showed, "Page not found" in place of a changeset an
 * undo found gone or of a page deleted under its editor (M12/P2/S1).
 */
export function useArrivalFocus<T extends HTMLElement>(also = false, replacing = false): RefObject<T | null> {
  const ref = useRef<T>(null);
  const location = useLocation();
  const type = useNavigationType();
  const arrival = also || isArrival(location.state);
  const fromNoOne = !arrival && (replacing || (type === "POP" && isOpened(location.state)));
  const entry = useRef<object>(location);
  useEffect(() => {
    entry.current = location;
  });
  useEffect(() => {
    if (arrival) {
      takeArrivalFocus(entry.current, ref.current);
    } else {
      offered.add(entry.current);
      if (fromNoOne) {
        // No longer its first: from no one only.
        takeArrivalFocus(entry.current, ref.current);
      }
    }
  }, [arrival, fromNoOne]);
  return ref;
}
