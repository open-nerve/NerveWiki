import type { ApiClient, BacklinkPage, LinkLanding, LinkTarget, PageProperties, TagCount } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { BacklinkPage, LinkLanding, LinkTarget, PageProperties, TagCount };

/**
 * LinkingService reads the link index (M6/P5, P6, P7): the pages of a tag,
 * where a page made for a link that leads nowhere would go, what the
 * editor completes, and a page's backlinks and properties.
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

  /** linkTargets answers what the notebook's links may lead to: its pages and attachments, each with its kind, link and aliases, by id. */
  async linkTargets(notebookId: string): Promise<LinkTarget[]> {
    const { data } = await unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/link-targets", {
        params: { path: { notebook_id: notebookId } },
      })
    );
    return data;
  }

  /** tags answers the notebook's tags, each with how many pages have it. */
  async tags(notebookId: string): Promise<TagCount[]> {
    const { data } = await unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/tags", { params: { path: { notebook_id: notebookId } } })
    );
    return data;
  }

  /** backlinks answers a page of the pages that link to the page id, after cursor (none for the first). */
  async backlinks(id: string, cursor?: string): Promise<BacklinkPage> {
    return unwrap(
      await this.api.GET("/api/v0/pages/{page_id}/backlinks", {
        params: { path: { page_id: id }, query: cursor === undefined ? {} : { cursor } },
      })
    );
  }

  /** properties answers the page id's properties, and where its property links lead. */
  async properties(id: string): Promise<PageProperties> {
    return unwrap(await this.api.GET("/api/v0/pages/{page_id}/properties", { params: { path: { page_id: id } } }));
  }
}
