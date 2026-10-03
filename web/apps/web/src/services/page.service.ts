import type { ApiClient, NodeMove, Page, PageView, TreeNode } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { NodeMove, PageView, TreeNode };

/**
 * PageService reads a notebook's page tree and a page's reading view, and
 * writes the tree: creates, renames, moves and deletes pages (M4/P5 design
 * 3.4).
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

  /** createPage creates a page titled title under parent (null: the root), last among its siblings. */
  async createPage(notebookId: string, parent: string | null, title: string): Promise<Page> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/pages", {
        params: { path: { notebook_id: notebookId } },
        body: { parent_id: parent, title },
      })
    );
  }

  async renameNode(id: string, name: string): Promise<TreeNode> {
    return unwrap(
      await this.api.PATCH("/api/v0/nodes/{node_id}", { params: { path: { node_id: id } }, body: { name } })
    );
  }

  async moveNode(id: string, move: NodeMove): Promise<TreeNode> {
    return unwrap(
      await this.api.POST("/api/v0/nodes/{node_id}/move", { params: { path: { node_id: id } }, body: move })
    );
  }

  /** deleteNode deletes the page id with the pages under it. */
  async deleteNode(id: string): Promise<void> {
    await unwrap(await this.api.DELETE("/api/v0/nodes/{node_id}", { params: { path: { node_id: id } } }));
  }

  /** getPageView answers the page's content rendered, with the revision it was rendered from. */
  async getPageView(id: string): Promise<PageView> {
    return unwrap(await this.api.GET("/api/v0/pages/{page_id}/view", { params: { path: { page_id: id } } }));
  }
}
