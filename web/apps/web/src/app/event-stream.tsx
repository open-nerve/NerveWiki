import { useEffect } from "react";
import { unstable_serialize, useSWRConfig, type Cache, type ScopedMutator } from "swr";

import type { HubEvent } from "../events/hub";
import type { Refresher } from "../events/refresher";
import type { PageView } from "../services/page.service";
import { useStore } from "../stores/context";
import { useSession } from "./guards";

/**
 * What each connection of the stream reads again, since what was written
 * while it was not connected went unseen: from the outside in, a level once
 * the one before it is read and shown, so that a workspace or a notebook
 * no longer seen, or a page deleted, leaves the page before what is in it
 * is read, which would be not found.
 */
const refreshedOnConnect = [["workspaces"], ["notebooks"], ["pages"], ["page-view", "edit-lock"]].map(
  (level) => new Set(level)
);

/**
 * EventStream shows what others write as they write it (M5/P3 design 3.8):
 * while the tab is signed in, this generation's hub runs, and each event
 * has SWR read again what it changed, which only what is mounted does: a
 * tree that changed, a page's reading view whose cached revision is older
 * than the one written (or not read yet), a page's edit lock after its
 * notebook's tree, and on each connection all of them with the workspaces
 * and notebooks. The reading views are read
 * through the refresher, at most once in its interval, and once visible
 * again when the tab is hidden. It sits with the providers, mounted anew
 * with each generation, whose hub stops with it: a refresh still going on
 * then reads no more, its cache gone with the generation.
 */
export function EventStream() {
  const store = useStore();
  const { status } = useSession();
  const { cache, mutate } = useSWRConfig();
  const signedIn = status === "signed-in";
  useEffect(() => {
    const hub = store.events();
    const refresher = signedIn ? store.newRefresher() : undefined;
    if (hub === undefined || refresher === undefined) {
      return undefined;
    }
    const stop = new AbortController();
    const unsubscribe = hub.subscribe((event) => route(event, cache, mutate, refresher, stop.signal));
    hub.start();
    return () => {
      stop.abort();
      unsubscribe();
      hub.stop();
      refresher.stop();
    };
  }, [store, signedIn, cache, mutate]);
  return null;
}

function route(event: HubEvent, cache: Cache, mutate: ScopedMutator, refresher: Refresher, stopped: AbortSignal): void {
  switch (event.type) {
    case "connected":
      void refreshAll(mutate, stopped);
      break;
    case "lock":
      void refreshLock(mutate, event.data.notebook_id, event.data.page_id, stopped);
      break;
    case "pages": {
      const { notebook_id: notebook, tree, pages } = event.data;
      if (tree) {
        void mutate(["pages", notebook]);
      }
      if (pages === null) {
        // Too many pages to name: every reading view of the notebook.
        refresher.request(`page-views ${notebook}`, () => {
          void mutate((key) => Array.isArray(key) && key[0] === "page-view" && key[1] === notebook);
        });
        break;
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
      break;
    }
  }
}

async function refreshAll(mutate: ScopedMutator, stopped: AbortSignal): Promise<void> {
  for (const level of refreshedOnConnect) {
    if (stopped.aborted) {
      return;
    }
    // oxlint-disable-next-line no-await-in-loop -- one level after another
    await mutate((key) => level.has((Array.isArray(key) ? key[0] : key) as string));
    // oxlint-disable-next-line no-await-in-loop -- one level after another
    await shown();
  }
}

/**
 * refreshLock reads the page's lock again, after the notebook's tree: the
 * session of a page deleted while edited ends before the deletion's event
 * comes, and its page leaves before its lock would be read, not found.
 */
async function refreshLock(mutate: ScopedMutator, notebook: string, page: string, stopped: AbortSignal) {
  await mutate(["pages", notebook]);
  await shown();
  if (!stopped.aborted) {
    await mutate(["edit-lock", page]);
  }
}

/** shown lets React show what was read, unmounting what is gone, before the next read. */
function shown(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}
