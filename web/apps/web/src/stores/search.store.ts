import type { PageService, SearchPage } from "../services/page.service";

/**
 * SearchStore searches one workspace for one generation (M8/P5 design
 * 4.3). It keeps nothing: a search's hits are in SWR's cache alone, under
 * the key of the search's entry in the tab's history, as a page's reading
 * view is (v0.1 design 13.2, item 7).
 */
export class SearchStore {
  constructor(
    private readonly service: Pick<PageService, "search">,
    /** The slug of the workspace it searches. */
    private readonly slug: string
  ) {}

  /** search reads a page of the hits of q, in the notebook notebookId or in all the account sees; after cursor. */
  search(q: string, notebookId: string | undefined, cursor?: string): Promise<SearchPage> {
    return this.service.search(this.slug, q, { notebookId, cursor });
  }
}
