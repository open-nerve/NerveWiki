import { createContext } from "react";
import { unstable_serialize, type Cache, type ScopedMutator } from "swr";

import type { EventLinks, EventLock, EventPages } from "../services/event.service";
import type { PageView } from "../services/page.service";
import type { StreamEvent } from "./channel";
import type { Refresher } from "./refresher";

// The app's handlers of the stream's events, by type (M5 design 8): each
// has SWR read again what an event of its type changed, which only what
// is mounted does. M5 has pages and lock, M6 links; a later M adds its
// type here (M10 and M11 a notebook's mode), the stream and the tabs
// passing on a type they do not know with its data.

/**
 * EventContext is what a handler has besides the event: SWR's cache and
 * mutate, the refresher that merges the reading views' re-reads, and
 * stopped, aborted once the stream's generation is over.
 */
type EventContext = { cache: Cache; mutate: ScopedMutator; refresher: Refresher; stopped: AbortSignal };

/** An EventHandler handles an event of its type: data is the event's, as the server sent it. */
export type EventHandler = (data: unknown, context: EventContext) => void;

/** typeOf is the type of an event of the stream, its handler's key. */
export function typeOf(event: StreamEvent): string {
  return event.type === "other" ? event.event : event.type;
}

/**
 * pagesChanged reads again a tree that changed and a page's reading view
 * whose cached revision is older than the one written (or not read yet),
 * through the refresher.
 */
const pagesChanged: EventHandler = (data, context) => {
  const { cache, mutate, refresher } = context;
  const { notebook_id: notebook, tree, pages } = data as EventPages;
  if (tree) {
    void mutate(["pages", notebook]);
  }
  if (pages === null) {
    readViews(notebook, context);
    return;
  }
  for (const { id, revision } of pages) {
    const key = ["page-view", notebook, id];
    // A view whose first read is out is read again, as that read may be the older; one not shown, or
    // left without data by the editing tab's save, is not.
    const state = cache.get(unstable_serialize(key));
    const cached = state?.data as PageView | undefined;
    if (cached === undefined ? state?.isLoading === true : cached.revision < revision) {
      refresher.request(unstable_serialize(key), () => void mutate(key));
    }
  }
};

/**
 * linksChanged reads again the reading views of the pages whose links lead
 * elsewhere (M6 design 4.8, M6/P3 design 6.7): at the revision shown, so
 * whatever that is, through the refresher; every view of the notebook for
 * too many pages to name. The pages whose backlinks changed, its targets,
 * are for the backlinks to read again.
 */
const linksChanged: EventHandler = (data, context) => {
  const { mutate, refresher } = context;
  const { notebook_id: notebook, pages } = data as EventLinks;
  if (pages === null) {
    readViews(notebook, context);
    return;
  }
  for (const id of pages) {
    const key = ["page-view", notebook, id];
    refresher.request(unstable_serialize(key), () => void mutate(key));
  }
};

/** readViews reads every reading view of notebook again, through the refresher: too many pages to name. */
function readViews(notebook: string, { mutate, refresher }: EventContext) {
  refresher.request(`page-views ${notebook}`, () => {
    void mutate((key) => Array.isArray(key) && key[0] === "page-view" && key[1] === notebook);
  });
}

/**
 * lockChanged reads the page's lock again, after the notebook's tree: the
 * session of a page deleted while edited ends before the deletion's event
 * comes, and its page leaves before its lock would be read, not found.
 */
const lockChanged: EventHandler = (data, { mutate, stopped }) => {
  const { notebook_id: notebook, page_id: page } = data as EventLock;
  void (async () => {
    await mutate(["pages", notebook]);
    await shown();
    if (!stopped.aborted) {
      await mutate(["edit-lock", page]);
    }
  })();
};

/**
 * eventHandlers are the app's handlers by event type. The app's
 * composition root (main.tsx) gives them to the event stream through
 * EventHandlers; without it an event is handled by none.
 */
export const eventHandlers: ReadonlyMap<string, EventHandler> = new Map([
  ["pages", pagesChanged],
  ["lock", lockChanged],
  ["links", linksChanged],
]);

export const EventHandlers = createContext<ReadonlyMap<string, EventHandler>>(new Map());

/** shown lets React show what was read, unmounting what is gone, before the next read. */
export function shown(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}
