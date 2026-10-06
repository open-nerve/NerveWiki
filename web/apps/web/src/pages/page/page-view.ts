import useSWR, { type SWRConfiguration } from "swr";

import type { Notebook } from "../../services/notebook.service";
import type { PageView } from "../../services/page.service";
import { usePageTree } from "../../stores/context";

/**
 * usePageView reads the page's reading view, by the key the events read it
 * again by (events/handlers.ts). Each of its readers reads it the same way:
 * SWR has the first of them read it again, whichever that is.
 */
export function usePageView(notebook: Notebook, page: string, config?: SWRConfiguration<PageView>) {
  const pages = usePageTree(notebook);
  return useSWR(["page-view", notebook.id, page], () => pages.view(page), config);
}
