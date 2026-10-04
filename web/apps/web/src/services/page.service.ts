import type {
  ApiClient,
  EditLock,
  EditSession,
  NodeMove,
  Page,
  PageContent,
  PageContentWrite,
  PageView,
  TaskToggle,
  TreeNode,
} from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { EditLock, NodeMove, PageContent, PageView, TaskToggle, TreeNode };

/**
 * PageService reads a notebook's page tree and a page's reading view, and
 * writes the tree: creates, renames, moves and deletes pages (M4/P5 design
 * 3.4). It reads and writes a page's content, in an edit session (M4/P6
 * design 3.6), and ticks its task items (M5/P6).
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

  /** getPageContent answers the page's content as written, with its revision. */
  async getPageContent(id: string): Promise<PageContent> {
    return unwrap(await this.api.GET("/api/v0/pages/{page_id}/content", { params: { path: { page_id: id } } }));
  }

  /** putPageContent writes the page's content on the revision it was read at; it answers the page, with its new revision. */
  async putPageContent(id: string, write: PageContentWrite): Promise<Page> {
    return unwrap(
      await this.api.PUT("/api/v0/pages/{page_id}/content", { params: { path: { page_id: id } }, body: write })
    );
  }

  /**
   * toggleTask ticks or clears the page's task item at the offset its checkbox carries, in the revision the
   * reading view was rendered from (M5/P6 design 3.4); it answers the page, with its new revision.
   */
  async toggleTask(id: string, toggle: TaskToggle): Promise<Page> {
    return unwrap(
      await this.api.POST("/api/v0/pages/{page_id}/toggle-task", { params: { path: { page_id: id } }, body: toggle })
    );
  }

  /** lock answers who holds the page's edit lock and for how many more seconds; both null when nobody does (M5 design 4.2). */
  async lock(id: string): Promise<EditLock> {
    return unwrap(await this.api.GET("/api/v0/pages/{page_id}/edit-lock", { params: { path: { page_id: id } } }));
  }

  /** releaseLock ends the session that holds the page's edit lock, if any: the notebook's admins may (M5 design 4.2). */
  async releaseLock(id: string): Promise<void> {
    await unwrap(await this.api.DELETE("/api/v0/pages/{page_id}/edit-lock", { params: { path: { page_id: id } } }));
  }

  /**
   * openEditSession opens the caller's edit session of the page, which holds its edit lock; with takeOver,
   * the caller's own sessions of it elsewhere end first (M5 design 4.1).
   */
  async openEditSession(id: string, takeOver = false): Promise<EditSession> {
    return unwrap(
      await this.api.POST("/api/v0/pages/{page_id}/edit-sessions", {
        params: { path: { page_id: id } },
        body: { take_over: takeOver },
      })
    );
  }

  /** heartbeatEditSession keeps the session id alive; signal gives the beat up. */
  async heartbeatEditSession(id: string, signal?: AbortSignal): Promise<EditSession> {
    return unwrap(
      await this.api.POST("/api/v0/edit-sessions/{edit_session_id}/heartbeat", {
        params: { path: { edit_session_id: id } },
        signal,
      })
    );
  }

  async endEditSession(id: string): Promise<void> {
    await unwrap(
      await this.api.DELETE("/api/v0/edit-sessions/{edit_session_id}", { params: { path: { edit_session_id: id } } })
    );
  }
}

/**
 * EditLeaveService ends an edit session as the page is left (M5 design
 * 4.7): in pagehide, where only what is sent synchronously goes out, with
 * keepalive so that the request outlives the page. Its client has no auth
 * middleware, which is asynchronous: the caller gives the token.
 */
export class EditLeaveService {
  constructor(private readonly api: ApiClient) {}

  /** endOnLeave sends the end of the session id now, with token; it waits for no answer and fails silently. */
  endOnLeave(id: string, token: string): void {
    this.api
      .DELETE("/api/v0/edit-sessions/{edit_session_id}", {
        params: { path: { edit_session_id: id } },
        headers: { Authorization: `Bearer ${token}` },
        keepalive: true,
      })
      .catch(() => undefined);
  }
}
