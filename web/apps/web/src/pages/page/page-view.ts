import useSWR from "swr";

import type { Notebook } from "../../services/notebook.service";
import type { PageView } from "../../services/page.service";
import { usePageTree } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { eachRead, stamped, useAssetsExpiry } from "./assets-expiry";

/**
 * usePageView reads the page's reading view, by the key the events read it
 * again by (events/handlers.ts). Its readers, the view and the outline,
 * read it the same way: SWR has one of them read it again, whichever
 * subscribed first, and reads it once for all. A view whose attachments'
 * addresses had expired as it came, the cache's, is none until it is read
 * again; one shown is read again before they do (M7/P4 design 4.5).
 * readView is how it is read, its time kept.
 */
export function usePageView(notebook: Notebook, page: string) {
  const pages = usePageTree(notebook);
  const { data, error, mutate } = useSWR(["page-view", notebook.id, page], () => readView(pages, page), eachRead);
  return { data: useAssetsExpiry(data, () => void mutate()), error, mutate };
}

/** readView reads the page id's reading view, as SWR has it: with when it came. */
export function readView(pages: PageTreeStore, id: string): Promise<PageView> {
  return stamped(pages.view(id));
}
