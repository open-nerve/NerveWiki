import useSWR from "swr";

import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import { useAssetsExpiry } from "./assets-expiry";

/**
 * usePageView reads the page's reading view, by the key the events read it
 * again by (events/handlers.ts). Its readers, the view and the outline,
 * read it the same way: SWR has one of them read it again, whichever
 * subscribed first, and reads it once for all. A view whose attachments'
 * addresses had expired as it came, the cache's, is none until it is read
 * again; one shown is read again before they do (M7/P4 design 4.5).
 */
export function usePageView(notebook: Notebook, page: string) {
  const pages = usePageTree(notebook);
  const { data, error, mutate } = useSWR(["page-view", notebook.id, page], () => pages.view(page));
  return { data: useAssetsExpiry(data, () => void mutate()), error, mutate };
}
