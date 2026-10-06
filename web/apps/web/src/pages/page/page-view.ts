import useSWR from "swr";

import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";

/**
 * usePageView reads the page's reading view, by the key the events read it
 * again by (events/handlers.ts). Its readers, the view and the outline,
 * read it the same way: SWR has one of them read it again, whichever
 * subscribed first, and reads it once for all.
 */
export function usePageView(notebook: Notebook, page: string) {
  const pages = usePageTree(notebook);
  return useSWR(["page-view", notebook.id, page], () => pages.view(page));
}
