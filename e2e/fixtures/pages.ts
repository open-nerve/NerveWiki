import type { ApiClient, Page, PageCreate, TreeNode } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The pages of the stories, through the API (M4 design 5).

/** credential's creation of the page body in the notebook notebookId, as the API answers it. */
export async function postPage(api: ApiClient, credential: string, notebookId: string, body: PageCreate) {
  return api.POST("/api/v0/notebooks/{notebook_id}/pages", {
    params: { path: { notebook_id: notebookId } },
    body,
    headers: bearer(credential),
  });
}

/** Creates the page titled title in the notebook notebookId with credential, under parentId or at the root, and returns it. */
export async function createPage(
  api: ApiClient,
  credential: string,
  notebookId: string,
  title: string,
  parentId: string | null = null
): Promise<Page> {
  const { data, error, response } = await postPage(api, credential, notebookId, { parent_id: parentId, title });
  expect(response.status, `create ${title}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`create ${title} answered 201 without the page`);
  }
  return data;
}

/** credential's rename of the node id to name, as the API answers it. */
export async function renameNode(api: ApiClient, credential: string, id: string, name: string) {
  return api.PATCH("/api/v0/nodes/{node_id}", {
    params: { path: { node_id: id } },
    body: { name },
    headers: bearer(credential),
  });
}

/** credential's read of the page id, as the API answers it. */
export async function getPage(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/pages/{page_id}", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}

/** credential's read of the tree of the notebook notebookId, as the API answers it. */
export async function getTree(api: ApiClient, credential: string, notebookId: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}/nodes", {
    params: { path: { notebook_id: notebookId } },
    headers: bearer(credential),
  });
}

/** The tree of the notebook notebookId that credential reads, each parent before its children. */
export async function listNodes(api: ApiClient, credential: string, notebookId: string): Promise<TreeNode[]> {
  const { data, error, response } = await getTree(api, credential, notebookId);
  expect(response.status, `list the tree: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}
