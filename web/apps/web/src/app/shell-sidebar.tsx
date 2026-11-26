import { PanelLeft } from "lucide-react";
import { FocusScope } from "radix-ui/internal";
import {
  createContext,
  use,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
  type RefObject,
} from "react";
import { createPath, useLocation, useNavigate, useNavigationType } from "react-router";

import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import { linkFollowed } from "../lib/link-click";
import { arrived, headingTakes, opened } from "./arrival";
import { useNarrow } from "./narrow";
import { composing } from "./shortcuts";
import { UserMenu } from "./user-menu";

/**
 * intoColumn brings element into view in the left column's middle ([data-sidebar-scroll]), 8px to spare above or
 * below it, by that area's scrollTop alone: never scrollIntoView, which scrolls the window too (v0.1 design 13.2,
 * item 26; M13/P2/S2, 4.2.5). It answers false for an element in no such area.
 */
export function intoColumn(element: Element): boolean {
  const area = element.closest<HTMLElement>("[data-sidebar-scroll]");
  if (area === null) {
    return false;
  }
  const row = element.getBoundingClientRect();
  const view = area.getBoundingClientRect();
  if (row.top < view.top) {
    area.scrollTop -= view.top - row.top + 8;
  } else if (row.bottom > view.bottom) {
    area.scrollTop += row.bottom - view.bottom + 8;
  }
  return true;
}

/**
 * What the drawer of a phone's width does, the same for the shell's life (decision 790): read as they are called,
 * the address and the drawer as the last commit left them. closed: whether it is a drawer, closed (below md);
 * open: opens it, the focus to its first control or, "search", to the search box, its text selected; close: closes
 * it, the focus to the menu (Escape, the overlay); quietClose: closes it, the focus where it is (an unsaved edit's
 * Stay, decision 799); inDrawer: whether it is open, the focus held in it.
 */
export type DrawerActions = {
  closed: () => boolean;
  open: (to: "first" | "search") => void;
  close: () => void;
  quietClose: () => void;
  inDrawer: () => boolean;
};

/** Where no shell gives its drawer (outside a workspace's): none, its actions doing nothing. */
const none: DrawerActions = {
  closed: () => false,
  open: () => {},
  close: () => {},
  quietClose: () => {},
  inDrawer: () => false,
};

/** Drawer is the shell's drawer's actions, for what is below it: the tree's links, an unsaved edit's guard. */
export const Drawer = createContext<DrawerActions>(none);

/** useDrawerActions is the actions of the drawer of the shell above, or of none outside one. */
export function useDrawerActions(): DrawerActions {
  return use(Drawer);
}

/**
 * useFollow is the click of a link of the left column that is no row of the tree's pages, to its address to: an LLM
 * Wiki's Directory and History rows, a notebook's places' icons. It is followed as the tree's pages are: arrived at
 * from the drawer open, the page's title then taking the focus; opened otherwise, and the address shown replaced,
 * decided as it is clicked, the drawer's state read then (review A's P2A2-M1 and S3A-Q1; decisions 782, 808). A
 * click the browser's (linkFollowed) is left to it.
 */
export function useFollow(): (to: string) => (event: MouseEvent<HTMLAnchorElement>) => void {
  const { inDrawer } = useDrawerActions();
  const location = useLocation();
  const navigate = useNavigate();
  return (to) => (event) => {
    if (linkFollowed(event) === undefined) {
      return;
    }
    event.preventDefault();
    const here = createPath(location) === to;
    void navigate(to, { state: inDrawer() && !here ? arrived : opened, replace: here });
  };
}

/** What a shell's drawer is as it renders, and its actions (useDrawer). */
export type ShellDrawer = {
  narrow: boolean;
  trapping: boolean;
  menu: RefObject<HTMLButtonElement | null>;
  root: RefObject<HTMLDivElement | null>;
  actions: DrawerActions;
};

/** The drawer open: the address it was opened at, its key, path and query; where the focus went in it. */
type Opened = { at: string; pathname: string; search: string; to: "first" | "search" };

/** What the drawer opened gives the focus to when it opens at its first control (the switcher). */
const focusable = 'button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * useDrawer is the left column's drawer at a phone's width (M13/P2/S3, decisions 781, 782, 790, 797, 799), of the
 * shell that calls it, an ancestor of its pages: its effects run after theirs. Open, it was opened at the address of
 * that commit (at), and it holds the focus while the address is that one (trapping): the navigation's render has it
 * closed already, its scope's trap taken down in the passive cleanups before the page's heading takes the arrival
 * focus. In the navigation's commit the focus leaves it, to the body (Chromium corrects a focus in an inert subtree
 * at its next render only); after the page's passive effects the state goes, and the menu takes the focus unless the
 * page takes it itself (headingTakes). Escape and the overlay close it, the menu taking the focus once the trap is
 * down; Escape is the IME's in a composition, and the browser's in a search box holding text, which it empties first
 * (one that does not, it closes after all). A wider window closes it, the focus left where it is; a narrower one gives
 * the menu a focus left in the column, which the drawer closed is inert to (review A's S3A-N1). A replace of the query
 * alone is the page's, no navigation away: a filter's choice waiting, used as the box takes the focus (useChoice), the
 * drawer stays open, at the new entry (decision 807). Open, the window does not scroll behind it (decision 809).
 */
export function useDrawer(): ShellDrawer {
  const narrow = useNarrow();
  const location = useLocation();
  const type = useNavigationType();
  const [drawer, setDrawer] = useState<Opened | null>(null);
  // A wider window closes it, not to open again as the window narrows: the width of the last render, as
  // search-box.tsx keeps the q it took (no effect: react/set-state-in-effect).
  const [wide, setWide] = useState(!narrow);
  if (wide === narrow) {
    setWide(!narrow);
    if (!narrow) {
      setDrawer(null);
    }
  }
  // A filter's choice used, the same path, another query, replaced (decision 807): the drawer moves to its entry, as the
  // width's change is taken (no effect). The page shown chosen again in the tree, the same query too, still leaves it.
  if (
    drawer !== null &&
    drawer.at !== location.key &&
    type === "REPLACE" &&
    drawer.pathname === location.pathname &&
    drawer.search !== location.search
  ) {
    setDrawer({ ...drawer, at: location.key, search: location.search });
  }
  const trapping = narrow && drawer !== null && drawer.at === location.key;
  const menu = useRef<HTMLButtonElement>(null);
  const root = useRef<HTMLDivElement>(null);
  const toMenu = useRef(false);
  const now = useRef({ location, trapping, narrow });
  useLayoutEffect(() => {
    now.current = { location, trapping, narrow };
  }, [location, trapping, narrow]);
  const actions = useMemo<DrawerActions>(
    () => ({
      closed: () => now.current.narrow && !now.current.trapping,
      open: (to) => {
        const { key, pathname, search } = now.current.location;
        setDrawer({ at: key, pathname, search, to });
      },
      close: () => {
        toMenu.current = true;
        setDrawer(null);
      },
      quietClose: () => setDrawer(null),
      inDrawer: () => now.current.trapping,
    }),
    []
  );
  // Narrower, the column turned an inert drawer: a focus left in it to the menu, not to the body (review A's S3A-N1),
  // the window not scrolled to the strip at its top: the reader did nothing, the window changed (S3A1-N1).
  useLayoutEffect(() => {
    if (narrow && root.current?.contains(document.activeElement)) {
      menu.current?.focus({ preventScroll: true });
    }
  }, [narrow]);
  // Open, the window does not scroll behind it: its overflow hidden, as it was again once closed (decision 809). Where
  // its scrollbar took room (classic scrollbars, the page overflowing), its gutter kept: the page behind does not shift
  // by its width (S3B1-N1). Not where it took none (overlay or hidden scrollbars, a page that fits): a gutter there
  // would shift it.
  useEffect(() => {
    if (!trapping) {
      return undefined;
    }
    const html = document.documentElement;
    const style = html.style;
    const was = { overflow: style.overflow, gutter: style.scrollbarGutter };
    const room = window.innerWidth > html.clientWidth;
    style.overflow = "hidden";
    if (room) {
      style.scrollbarGutter = "stable";
    }
    return () => {
      style.overflow = was.overflow;
      style.scrollbarGutter = was.gutter;
    };
  }, [trapping]);
  // The navigation's commit: the focus leaves the drawer now, no setState (decision 790).
  useLayoutEffect(() => {
    const active = document.activeElement;
    if (
      drawer !== null &&
      drawer.at !== location.key &&
      active instanceof HTMLElement &&
      root.current?.contains(active)
    ) {
      active.blur();
    }
  }, [drawer, location.key]);
  // After the page's passive effects: the entry left, its state cleared; the focus to the menu unless the page takes it.
  useEffect(() => {
    if (drawer !== null && drawer.at !== location.key) {
      // oxlint-disable-next-line react/set-state-in-effect -- the entry left, after the page's effects: 02-P2 4.4
      setDrawer((open) => (open !== null && open.at !== location.key ? null : open));
      if (!headingTakes(location.state, type)) {
        menu.current?.focus();
      }
    }
  }, [drawer, location.key, location.state, type]);
  const to = drawer?.to;
  useEffect(() => {
    if (trapping) {
      const element = root.current;
      if (to === "search") {
        const box = element?.querySelector<HTMLInputElement>('input[type="search"]');
        box?.focus();
        box?.select();
      } else {
        element?.querySelector<HTMLElement>(focusable)?.focus();
      }
    } else if (toMenu.current) {
      toMenu.current = false;
      menu.current?.focus();
    }
  }, [trapping, to]);
  useEffect(() => {
    const element = root.current;
    if (!trapping || element === null) {
      return undefined;
    }
    // On the root itself: a menu or a dialog open in the drawer is a portal in the body, its Escape not this one's.
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || composing(event)) {
        return;
      }
      const at = event.target;
      if (at instanceof HTMLInputElement && at.type === "search" && at.value !== "") {
        // The browser's, which empties the box after the key's dispatch; one that does not, the drawer's after all.
        setTimeout(() => {
          if (at.value !== "" && now.current.trapping) {
            actions.close();
          }
        });
        return;
      }
      event.preventDefault();
      actions.close();
    };
    element.addEventListener("keydown", onKey);
    return () => element.removeEventListener("keydown", onKey);
  }, [trapping, actions]);
  return { narrow, trapping, menu, root, actions };
}

/** The drawer's scope takes no focus as it mounts and gives none back as it goes: the page's headings have it. */
function keep(event: Event): void {
  event.preventDefault();
}

/**
 * ShellSidebar is a shell's left column (M13/P2/S1, decision 774), in
 * three parts: its top, which stays (the workspace's switcher and search);
 * its middle, which scrolls itself ([data-sidebar-scroll]), the end of its
 * scroll carrying the window on, as before the column stuck: contained,
 * Chromium let no wheel over a middle that does not overflow scroll the
 * window (decision 802; review A, S1A-M2); its foot, the
 * account's row, above a line, the middle scrolling past it. On the
 * sidebar's colour, a shade off the page. From md: up it sticks to the
 * window's left, as tall as the window, which still scrolls the whole
 * page (00-M13 decision 717). It is no scroll anchor of the window's
 * (overflow-anchor: none), as when it was as tall as the page beside it,
 * which what changes size above the reader would move (M6/P6 B fix check
 * 4): stuck, a probe of the three engines saw no difference either way,
 * harmless kept (M13/P2/S1).
 *
 * Below md it is a drawer, the same element (M13/P2/S3, decisions 781,
 * 784): closed, inert, hidden and off the screen; open, its middle's end
 * carrying no scroll on (decision 809); opened by the menu of a
 * strip at the page's top, which scrolls with the page and says where the
 * page is (place), it slides in (not asked for less motion; it closes at
 * once) over an overlay, its edge a line as from md: up (forced colours
 * take the overlay and the shadow away), a dialog named Navigation that
 * holds the focus (Radix's FocusScope, always around it, its trap
 * switched: mounted anew, the column would mount anew). The strip is outside [data-shell].
 * Its root takes no focus on a click from md: up (no tabindex unless it
 * holds the focus).
 */
export function ShellSidebar({
  drawer,
  place,
  top,
  children,
}: {
  drawer: ShellDrawer;
  place: ReactNode;
  top: ReactNode;
  children: ReactNode;
}) {
  const t = useT();
  const id = useId();
  const { narrow, trapping, menu, root, actions } = drawer;
  return (
    <>
      <div className="flex h-11 shrink-0 items-center gap-1.5 border-b border-border bg-background px-2 md:hidden">
        <Button
          ref={menu}
          variant="ghost"
          size="icon-sm"
          aria-label={t("shell.menu")}
          aria-expanded={trapping}
          aria-controls={id}
          onClick={() => actions.open("first")}
        >
          <PanelLeft />
        </Button>
        <span className="min-w-0 truncate text-compact font-medium text-foreground">{place}</span>
      </div>
      <FocusScope.Root
        asChild
        trapped={trapping}
        loop={trapping}
        tabIndex={trapping ? -1 : undefined}
        onMountAutoFocus={keep}
        onUnmountAutoFocus={keep}
      >
        <div
          ref={root}
          id={id}
          data-shell
          data-drawer={narrow ? (trapping ? "open" : "closed") : undefined}
          inert={narrow && !trapping}
          role={trapping ? "dialog" : undefined}
          aria-modal={trapping || undefined}
          aria-label={trapping ? t("shell.menu") : undefined}
          className="flex flex-col bg-sidebar outline-none [overflow-anchor:none] max-md:fixed max-md:inset-y-0 max-md:left-0 max-md:z-40 max-md:w-63 max-md:max-w-[85vw] max-md:border-r max-md:border-border max-md:shadow-dialog max-md:data-[drawer=closed]:invisible max-md:data-[drawer=closed]:-translate-x-full max-md:data-[drawer=open]:motion-safe:transition-[translate] md:sticky md:top-0 md:h-svh md:w-63 md:shrink-0 md:self-start md:border-r md:border-border"
        >
          <div className="space-y-2 px-2 pt-2.5 pb-1.5">{top}</div>
          {/* pt-1: the area clips what overflows it, the first link's focus ring 2px out of it included. Scrolling, it is
              no stop of Tab, which Firefox makes it, nameless (M13 closeout, review A's M13CA-M3): the focus in it has
              PageDown scroll it still. */}
          <div
            data-sidebar-scroll
            tabIndex={-1}
            className="min-h-0 flex-1 overflow-y-auto px-2 pt-1 pb-3 max-md:overscroll-contain"
          >
            {children}
          </div>
          <div className="shrink-0 border-t border-border p-2">
            <UserMenu place="row" />
          </div>
        </div>
      </FocusScope.Root>
      {trapping && (
        <div
          aria-hidden
          className="fixed inset-0 z-30 animate-fade-in bg-overlay md:hidden"
          onClick={() => actions.close()}
        />
      )}
    </>
  );
}
