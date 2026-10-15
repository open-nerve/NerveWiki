import type { ApiClient } from "@nervewiki/api-client";

import { bearer } from "./auth";

// A workspace's search through the API (M8 design 4.12; M8/P5 design
// 4.6). fixtures/wiki-search.ts has its page as a user works it.

/** What a search is narrowed by, as its query takes it: the notebook, a page's cursor and size. */
export interface SearchQuery {
  notebookId?: string;
  cursor?: string;
  limit?: number;
}

/** credential's search of q in the workspace of slug, as query narrows it, as the API answers it. */
export async function search(api: ApiClient, credential: string, slug: string, q: string, query: SearchQuery = {}) {
  return api.GET("/api/v0/workspaces/{slug}/search", {
    params: {
      path: { slug },
      query: { q, notebook_id: query.notebookId, cursor: query.cursor, limit: query.limit },
    },
    headers: bearer(credential),
  });
}
