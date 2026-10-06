import type { ApiClient, LinkLanding } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { LinkLanding };

/**
 * LinkingService reads the link index (M6/P5, P6): the pages of a tag, and
 * where a page made for a link that leads nowhere would go.
 */
export class LinkingService {
  constructor(private readonly api: ApiClient) {}

  /** tagPages answers the ids of the pages of the notebook that have tag, or a tag under it (tag/…), by id. */
  async tagPages(notebookId: string, tag: string): Promise<string[]> {
    const { data } = await unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/tags/{tag}", {
        params: { path: { notebook_id: notebookId, tag } },
      })
    );
    return data.map((page) => page.id);
  }

  /**
   * linkLanding answers where a page made for target, a link's target on the page id as its view carries it
   * (data-nw-target), would go so that the link leads to it: the page it leads to already, a parent and a title,
   * or why there is none (M6/P6 design 2). Only a writer of the notebook's pages may ask.
   */
  async linkLanding(id: string, target: string): Promise<LinkLanding> {
    return unwrap(
      await this.api.GET("/api/v0/pages/{page_id}/link-landing", {
        params: { path: { page_id: id }, query: { target } },
      })
    );
  }
}
