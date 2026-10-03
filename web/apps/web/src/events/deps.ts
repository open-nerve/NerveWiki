import type { Port } from "./channel";
import type { PageLifecycle } from "./hub";
import type { Locks } from "./leadership";

/**
 * What the event stream needs of the browser (M5 design 4.11): Web Locks
 * where the page has them, else the storage the tabs share, with its
 * changes in other tabs; the channels between tabs; the page's lifecycle;
 * the clock and this tab's id.
 */
export type EventDeps = {
  locks: Locks | undefined;
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">;
  onStorage: (listener: (key: string | null) => void) => () => void;
  channel: (name: string) => Port;
  page: PageLifecycle;
  now: () => number;
  tabId: string;
};

/**
 * browserEventDeps are the event stream's dependencies in this tab, over
 * the storage and storage events the session uses (the session's own
 * memory storage, where site data is blocked, no tab shares: each tab then
 * holds a stream).
 */
export function browserEventDeps(
  storage: EventDeps["storage"],
  onStorage: EventDeps["onStorage"],
  tabId: string
): EventDeps {
  return {
    locks: "locks" in navigator ? (navigator.locks as Locks) : undefined,
    storage,
    onStorage,
    channel: (name) => new BroadcastChannel(name),
    page: {
      visible: () => document.visibilityState === "visible",
      on: (event, listener) => {
        const target = event === "pagehide" || event === "pageshow" ? window : document;
        target.addEventListener(event, listener);
        return () => target.removeEventListener(event, listener);
      },
    },
    now: Date.now,
    tabId,
  };
}
