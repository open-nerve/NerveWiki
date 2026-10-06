import { useContext, useEffect } from "react";
import { useSWRConfig, type ScopedMutator } from "swr";

import { EventHandlers, shown, typeOf } from "../events/handlers";
import { useStore } from "../stores/context";
import { useSession } from "./guards";

/**
 * What each connection of the stream reads again, since what was written
 * while it was not connected went unseen: from the outside in, a level once
 * the one before it is read and shown, so that a workspace or a notebook
 * no longer seen, or a page deleted, leaves the page before what is in it
 * is read, which would be not found.
 */
const refreshedOnConnect = [
  ["workspaces"],
  ["notebooks"],
  ["pages", "tag-pages"],
  ["page-view", "edit-lock", "backlinks", "page-properties"],
].map((level) => new Set(level));

/**
 * EventStream shows what others write as they write it (M5/P3 design 3.8):
 * while the tab is signed in, this generation's hub runs, and each event
 * goes to the app's handler of its type (events/handlers.ts), which has
 * SWR read again what it changed; on each connection all of what is shown
 * is read again, the workspaces and notebooks with it. The reading views
 * are read through the refresher, at most once in its interval, and once
 * visible again when the tab is hidden. It sits with the providers,
 * mounted anew with each generation, whose hub stops with it: a refresh
 * still going on then reads no more, its cache gone with the generation.
 */
export function EventStream() {
  const store = useStore();
  const { status } = useSession();
  const { cache, mutate } = useSWRConfig();
  const handlers = useContext(EventHandlers);
  const signedIn = status === "signed-in";
  useEffect(() => {
    const hub = store.events();
    const refresher = signedIn ? store.newRefresher() : undefined;
    if (hub === undefined || refresher === undefined) {
      return undefined;
    }
    const stop = new AbortController();
    const context = { cache, mutate, refresher, stopped: stop.signal };
    const unsubscribe = hub.subscribe((event) => {
      if (event.type === "connected") {
        void refreshAll(mutate, stop.signal);
        return;
      }
      handlers.get(typeOf(event))?.(event.data, context);
    });
    hub.start();
    return () => {
      stop.abort();
      unsubscribe();
      hub.stop();
      refresher.stop();
    };
  }, [store, signedIn, cache, mutate, handlers]);
  return null;
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
