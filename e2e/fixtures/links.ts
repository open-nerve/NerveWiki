import type { ApiClient } from "@nervewiki/api-client";

import { bearer } from "./auth";

// The links of the stories, through the API (M6 design 5).

/** credential's read of the pages that link to the page id, a page of them at a time, as the API answers it. */
export async function listBacklinks(
  api: ApiClient,
  credential: string,
  id: string,
  query: { limit?: number; cursor?: string } = {}
) {
  return api.GET("/api/v0/pages/{page_id}/backlinks", {
    params: { path: { page_id: id }, query },
    headers: bearer(credential),
  });
}

/** credential's read of the page id's properties and their links, as the API answers it. */
export async function getPageProperties(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/pages/{page_id}/properties", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}

/** credential's read of the tags of the notebook notebookId, as the API answers it. */
export async function listTags(api: ApiClient, credential: string, notebookId: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}/tags", {
    params: { path: { notebook_id: notebookId } },
    headers: bearer(credential),
  });
}

/** credential's read of the pages tagged tag, or a tag under it, in the notebook notebookId, as the API answers it. */
export async function getTag(api: ApiClient, credential: string, notebookId: string, tag: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}/tags/{tag}", {
    params: { path: { notebook_id: notebookId, tag } },
    headers: bearer(credential),
  });
}

/** credential's read of what the links of the notebook notebookId may lead to, as the API answers it. */
export async function listLinkTargets(api: ApiClient, credential: string, notebookId: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}/link-targets", {
    params: { path: { notebook_id: notebookId } },
    headers: bearer(credential),
  });
}

/** credential's read of where a page made for the link target, written in the page id, would go. */
export async function getLinkLanding(api: ApiClient, credential: string, id: string, target: string) {
  return api.GET("/api/v0/pages/{page_id}/link-landing", {
    params: { path: { page_id: id }, query: { target } },
    headers: bearer(credential),
  });
}
