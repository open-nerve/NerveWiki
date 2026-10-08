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
 * TREE_INTERVAL_MS is how often a tab reads a notebook's tree at most: a
 * run of uploads, a unit each, reads it a few times, and another's change
 * shows within half a second (M7/P2 review C1).
 */
export const TREE_INTERVAL_MS = 500;

/**
 * pagesChanged reads again a tree that changed, through the refresher, at
 * most once in TREE_INTERVAL_MS: an upload is a unit of its own (M7/P2
 * design 3.10); a page's reading view whose cached revision is
 * older than the one written (or not read yet), through the refresher
 * too; the pages of the notebook's tags, which a page written or deleted
 * may join or leave (M6 design 4.8); and the properties of the pages
 * written, through the refresher (M6/P7 design 11).
 */
const pagesChanged: EventHandler = (data, context) => {
  const { cache, mutate, refresher } = context;
  const { notebook_id: notebook, tree, pages } = data as EventPages;
  if (tree) {
    refresher.request(
      unstable_serialize(["pages", notebook]),
      () => void mutate(["pages", notebook]),
      TREE_INTERVAL_MS
    );
  }
  if (tree || pages === null || pages.length > 0) {
    readTagPages(notebook, context);
  }
  readEach("page-properties", notebook, pages === null ? null : pages.map(({ id }) => id), context);
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
 * too many pages to name, and its tags' pages, as after a reindex. Their
 * properties too, whose links lead elsewhere as well; and the backlinks of
 * the pages whose backlinks changed, its targets (M6/P7 design 11).
 */
const linksChanged: EventHandler = (data, context) => {
  const { cache, mutate, refresher } = context;
  const { notebook_id: notebook, pages, targets } = data as EventLinks;
  readEach("backlinks", notebook, targets, context);
  readEach("page-properties", notebook, pages, context);
  if (pages === null) {
    readViews(notebook, context);
    readTagPages(notebook, context);
    return;
  }
  for (const id of pages) {
    const key = ["page-view", notebook, id];
    // Of a view never read, nothing: the refresher keeps each key it is asked for.
    if (cache.get(unstable_serialize(key)) !== undefined) {
      refresher.request(unstable_serialize(key), () => void mutate(key));
    }
  }
};

/**
 * readEach reads again what of kind, a page's backlinks or properties, is
 * shown of the pages ids of notebook, each through the refresher; of every
 * page of the notebook for null, too many to name. Of a page never read, it
 * reads nothing (one whose first read is out was: SWR writes its key as it
 * mounts): the refresher keeps each key it is asked for.
 */
function readEach(
  kind: "backlinks" | "page-properties",
  notebook: string,
  ids: readonly string[] | null,
  { cache, mutate, refresher }: EventContext
) {
  if (ids === null) {
    refresher.request(`${kind} ${notebook}`, () => {
      void mutate((key) => Array.isArray(key) && key[0] === kind && key[1] === notebook);
    });
    return;
  }
  for (const id of ids) {
    const key = [kind, notebook, id];
    if (cache.get(unstable_serialize(key)) !== undefined) {
      refresher.request(unstable_serialize(key), () => void mutate(key));
    }
  }
}

/** readViews reads every reading view of notebook again, through the refresher: too many pages to name. */
function readViews(notebook: string, { mutate, refresher }: EventContext) {
  refresher.request(`page-views ${notebook}`, () => {
    void mutate((key) => Array.isArray(key) && key[0] === "page-view" && key[1] === notebook);
  });
}

/** readTagPages reads the pages of each tag of notebook shown again, through the refresher. */
function readTagPages(notebook: string, { mutate, refresher }: EventContext) {
  refresher.request(`tag-pages ${notebook}`, () => {
    void mutate((key) => Array.isArray(key) && key[0] === "tag-pages" && key[1] === notebook);
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
