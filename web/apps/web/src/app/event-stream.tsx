import { useEffect } from "react";
import { unstable_serialize, useSWRConfig, type Cache, type ScopedMutator } from "swr";

import type { HubEvent } from "../events/hub";
import type { Refresher } from "../events/refresher";
import type { PageView } from "../services/page.service";
import { useStore } from "../stores/context";
import { useSession } from "./guards";

/** The lists that each connection of the stream reads again: what was written while it was not connected went unseen. */
const refreshedOnConnect = new Set(["notebooks", "pages", "page-view", "edit-lock"]);

/**
 * EventStream shows what others write as they write it (M5/P3 design 3.8):
 * while the tab is signed in, this generation's hub runs, and each event
 * has SWR read again what it changed, which only what is mounted does: a
 * tree that changed, a page's reading view whose cached revision is older
 * than the one written, a page's edit lock, and on each connection all of
 * them with the workspaces and notebooks. The reading views are read
 * through the refresher, at most once in its interval, and once visible
 * again when the tab is hidden. It sits with the providers, mounted anew
 * with each generation, whose hub stops with it.
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
    const unsubscribe = hub.subscribe((event) => route(event, cache, mutate, refresher));
    hub.start();
    return () => {
      unsubscribe();
      hub.stop();
      refresher.stop();
    };
  }, [store, signedIn, cache, mutate]);
  return null;
}

function route(event: HubEvent, cache: Cache, mutate: ScopedMutator, refresher: Refresher): void {
  switch (event.type) {
    case "connected":
      void mutate("workspaces");
      void mutate((key) => Array.isArray(key) && refreshedOnConnect.has(key[0] as string));
      break;
    case "lock":
      void mutate(["edit-lock", event.data.page_id]);
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
        const cached = cache.get(unstable_serialize(key))?.data as PageView | undefined;
        if (cached !== undefined && cached.revision < revision) {
          refresher.request(unstable_serialize(key), () => void mutate(key));
        }
      }
      break;
    }
  }
}
