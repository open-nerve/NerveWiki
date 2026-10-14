import type { ApiClient } from "@nervewiki/api-client";

import { bearer } from "./auth";

// A notebook's history through the API (M8 design 4.7, 4.8; M8/P3 design
// 4.8): its log, a changeset, its changes, and a revert of it.
// fixtures/wiki-pages.ts has its pages as a user works them.

/** What a log is filtered by, as its query takes it: the account, the client and the node; a page's size and cursor. */
export interface LogQuery {
  actor?: string;
  client?: string;
  pageId?: string;
  limit?: number;
  cursor?: string;
}

/** credential's read of a page of the notebook notebookId's log, as query filters it, as the API answers it. */
export async function listChangesets(api: ApiClient, credential: string, notebookId: string, query: LogQuery = {}) {
  return api.GET("/api/v0/notebooks/{notebook_id}/changesets", {
    params: {
      path: { notebook_id: notebookId },
      query: {
        actor: query.actor,
        client: query.client,
        page_id: query.pageId,
        limit: query.limit,
        cursor: query.cursor,
      },
    },
    headers: bearer(credential),
  });
}

/** credential's read of the changeset id, as the API answers it. */
export async function getChangeset(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/changesets/{changeset_id}", {
    params: { path: { changeset_id: id } },
    headers: bearer(credential),
  });
}

/** credential's read of a page of the changeset id's changes, after cursor, as the API answers it. */
export async function listChanges(api: ApiClient, credential: string, id: string, cursor?: string) {
  return api.GET("/api/v0/changesets/{changeset_id}/changes", {
    params: { path: { changeset_id: id }, query: cursor === undefined ? {} : { cursor } },
    headers: bearer(credential),
  });
}

/** credential's revert of the changeset id, with message or with no body, as the API answers it. */
export async function revertChangeset(api: ApiClient, credential: string, id: string, message?: string) {
  return api.POST("/api/v0/changesets/{changeset_id}/revert", {
    params: { path: { changeset_id: id } },
    ...(message === undefined ? {} : { body: { message } }),
    headers: bearer(credential),
  });
}
