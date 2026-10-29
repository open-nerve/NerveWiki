import type {
  ApiClient,
  Changeset,
  ChangesetBrief,
  ChangesetChange,
  ChangesetChangePage,
  ChangesetDetail,
  ChangesetPage,
  ChangesetReverted,
  EditLock,
  EditSession,
  NodeMove,
  NodeRestored,
  Page,
  PageContent,
  PageContentWrite,
  PageVersion,
  PageVersionContent,
  PageVersionPage,
  PageView,
  RevertConflict,
  SearchCount,
  SearchHit,
  SearchPage,
  TaskToggle,
  TextRange,
  TrashItem,
  TrashItemPage,
  TreeNode,
  UserRef,
  VersionRestored,
} from "@nervewiki/api-client";

import { unwrap } from "./api";

export type {
  Changeset,
  ChangesetBrief,
  ChangesetChange,
  ChangesetChangePage,
  ChangesetDetail,
  ChangesetPage,
  ChangesetReverted,
  EditLock,
  NodeMove,
  NodeRestored,
  Page,
  PageContent,
  PageVersion,
  PageVersionContent,
  PageVersionPage,
  PageView,
  RevertConflict,
  SearchCount,
  SearchHit,
  SearchPage,
  TaskToggle,
  TextRange,
  TrashItem,
  TrashItemPage,
  TreeNode,
  VersionRestored,
};

/** EditLockHolder is the account whose edit session holds a page's lock. */
export type EditLockHolder = UserRef;

/** How many of a page's versions a page of them holds: the right column's (M8/P1 design 4.3). */
const versionsPage = 20;

/** How many trash items a page of them holds: the trash's (M8/P2 design 4.2). */
const trashPage = 50;

/** How many changesets a page of a notebook's history holds: the history page's (M8/P3 design 4.2). */
const changesetsPage = 50;

/** How many changes a page of a changeset's holds: its page's (M8/P3 design 4.3). */
const changesPage = 100;

/** How many hits a page of a search holds at most: the server's default (M8/P5 design 4.1). */
const searchPage = 20;

/**
 * LogFilter narrows a notebook's history (M8 design 4.7): one account's
 * changesets (actor, its id), one client's (web, api, cli, mcp:<name>, or
 * mcp for every MCP client's), those that changed a node (page, a page's
 * or an attachment's id); "" for none.
 */
export type LogFilter = { actor: string; client: string; page: string };

/**
 * PageService reads a notebook's page tree and a page's reading view, and
 * writes the tree: creates, renames, moves and deletes pages (M4/P5 design
 * 3.4). It reads and writes a page's content, in an edit session (M4/P6
 * design 3.6), and ticks its task items (M5/P6). It reads a page's
 * versions, and restores one (M8/P1 design 3.6); it reads a notebook's
 * trash and an item of it, and restores one (M8/P2 design 4.1); it reads a
 * notebook's history, and reverts a changeset of it (M8/P3 design 4.1); it
 * searches a workspace (M8/P5 design 4.1).
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

  /** createPage creates a titled page and optional initial content in one request, last under parent. */
  async createPage(
    notebookId: string,
    parent: string | null,
    title: string,
    options?: { content?: string; signal?: AbortSignal }
  ): Promise<Page> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/pages", {
        params: { path: { notebook_id: notebookId } },
        body: { parent_id: parent, title, content: options?.content },
        signal: options?.signal,
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

  /** versions answers a page of the page id's versions, newer first, after cursor (none for the first). */
  async versions(id: string, cursor?: string): Promise<PageVersionPage> {
    return unwrap(
      await this.api.GET("/api/v0/pages/{page_id}/versions", {
        params: {
          path: { page_id: id },
          query: cursor === undefined ? { limit: versionsPage } : { limit: versionsPage, cursor },
        },
      })
    );
  }

  /**
   * version answers the version of the page id that holds revision, with its content: its own revision, which is a
   * later one when an edit session's saves made it hold several (M8 design 4.3).
   */
  async version(id: string, revision: number): Promise<PageVersionContent> {
    return unwrap(
      await this.api.GET("/api/v0/pages/{page_id}/versions/{revision}", {
        params: { path: { page_id: id, revision } },
      })
    );
  }

  /** versionView answers the reading view of the version of the page id that holds revision, at its own revision. */
  async versionView(id: string, revision: number): Promise<PageView> {
    return unwrap(
      await this.api.GET("/api/v0/pages/{page_id}/versions/{revision}/view", {
        params: { path: { page_id: id, revision } },
      })
    );
  }

  /**
   * restoreVersion writes the content of the version of the page id that holds revision as the page's, on base, the
   * revision the page was read at (M8 design 4.4); it answers the page's revision as it leaves it, and whether it
   * wrote it.
   */
  async restoreVersion(id: string, revision: number, base: number): Promise<VersionRestored> {
    return unwrap(
      await this.api.POST("/api/v0/pages/{page_id}/versions/{revision}/restore", {
        params: { path: { page_id: id, revision } },
        body: { base_revision: base },
      })
    );
  }

  /** trash answers a page of the notebook's trash items, newer first, after cursor (none for the first). */
  async trash(notebookId: string, cursor?: string): Promise<TrashItemPage> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/trash", {
        params: {
          path: { notebook_id: notebookId },
          query: cursor === undefined ? { limit: trashPage } : { limit: trashPage, cursor },
        },
      })
    );
  }

  /** trashItem answers the trash item whose root is the node id, as the trash lists it, with its notebook. */
  async trashItem(id: string): Promise<TrashItem> {
    return unwrap(await this.api.GET("/api/v0/nodes/{node_id}/trash-item", { params: { path: { node_id: id } } }));
  }

  /**
   * restoreNode brings back the trash item whose root is the node id, with the nodes deleted with it (M8 design
   * 4.6); it answers where it put it, and under which name.
   */
  async restoreNode(id: string): Promise<NodeRestored> {
    return unwrap(await this.api.POST("/api/v0/nodes/{node_id}/restore", { params: { path: { node_id: id } } }));
  }

  /**
   * changesets answers a page of the notebook's history that filter keeps, newer first by when each changeset was
   * first written, after cursor (none for the first).
   */
  async changesets(notebookId: string, filter: LogFilter, cursor?: string): Promise<ChangesetPage> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/changesets", {
        params: {
          path: { notebook_id: notebookId },
          query: {
            limit: changesetsPage,
            ...(filter.actor === "" ? {} : { actor: filter.actor }),
            ...(filter.client === "" ? {} : { client: filter.client }),
            ...(filter.page === "" ? {} : { page_id: filter.page }),
            ...(cursor === undefined ? {} : { cursor }),
          },
        },
      })
    );
  }

  /** changeset answers the changeset id as the history lists it, with whether its first changes rename or move a node. */
  async changeset(id: string): Promise<ChangesetDetail> {
    return unwrap(await this.api.GET("/api/v0/changesets/{changeset_id}", { params: { path: { changeset_id: id } } }));
  }

  /** changes answers a page of the changeset id's changes, by node id, after cursor (none for the first). */
  async changes(id: string, cursor?: string): Promise<ChangesetChangePage> {
    return unwrap(
      await this.api.GET("/api/v0/changesets/{changeset_id}/changes", {
        params: {
          path: { changeset_id: id },
          query: cursor === undefined ? { limit: changesPage } : { limit: changesPage, cursor },
        },
      })
    );
  }

  /**
   * revertChangeset puts every node the changeset id changed back as it was before it (M8 design 4.8), in a
   * changeset of its own, a revert, with message, or without one; it answers that changeset, and whether it wrote
   * anything.
   */
  async revertChangeset(id: string, message?: string): Promise<ChangesetReverted> {
    return unwrap(
      await this.api.POST("/api/v0/changesets/{changeset_id}/revert", {
        params: { path: { changeset_id: id } },
        ...(message === undefined ? {} : { body: { message } }),
      })
    );
  }

  /**
   * search answers a page of the hits of q in the workspace of slug (M8 design 4.12): in the notebook notebookId, or
   * in every notebook the caller sees without one; the first, or the one after cursor. The hits whose names hold
   * every term come first, then the pages whose names or contents hold each; a page may hold fewer hits than its
   * size, or none, with a cursor to go on with.
   */
  async search(
    slug: string,
    q: string,
    { notebookId, cursor }: { notebookId?: string; cursor?: string } = {}
  ): Promise<SearchPage> {
    return unwrap(
      await this.api.GET("/api/v0/workspaces/{slug}/search", {
        params: {
          path: { slug },
          query: {
            q,
            limit: searchPage,
            ...(notebookId === undefined ? {} : { notebook_id: notebookId }),
            ...(cursor === undefined ? {} : { cursor }),
          },
        },
      })
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
