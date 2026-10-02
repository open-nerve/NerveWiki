import type { ApiClient, PageView, TreeNode } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { PageView, TreeNode };

/**
 * PageService reads a notebook's page tree and a page's reading view (M4/P5
 * design 3.4).
 */
export class PageService {
  constructor(private readonly api: ApiClient) {}

  /** listNodes answers the notebook's whole tree: parents before their children, siblings in their order. */
  async listNodes(notebookId: string): Promise<TreeNode[]> {
    return (
      await unwrap(
        await this.api.GET("/api/v0/notebooks/{notebook_id}/nodes", { params: { path: { notebook_id: notebookId } } })
      )
    ).data;
  }

  /** getPageView answers the page's content rendered, with the revision it was rendered from. */
  async getPageView(id: string): Promise<PageView> {
    return unwrap(await this.api.GET("/api/v0/pages/{page_id}/view", { params: { path: { page_id: id } } }));
  }
}
