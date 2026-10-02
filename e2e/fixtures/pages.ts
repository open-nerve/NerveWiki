import type { ApiClient, NodeMove, Page, PageCreate, TreeNode } from "@nervewiki/api-client";
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

/** credential's move of the node id to body, as the API answers it. */
export async function postMove(api: ApiClient, credential: string, id: string, body: NodeMove) {
  return api.POST("/api/v0/nodes/{node_id}/move", {
    params: { path: { node_id: id } },
    body,
    headers: bearer(credential),
  });
}

/** Moves the node id to body with credential, and returns it as the API answers it. */
export async function moveNode(api: ApiClient, credential: string, id: string, body: NodeMove): Promise<TreeNode> {
  const { data, error, response } = await postMove(api, credential, id, body);
  expect(response.status, `move ${id}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`move ${id} answered 200 without the node`);
  }
  return data;
}

/** credential's deletion of the node id with its subtree, as the API answers it. */
export async function deleteNode(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/nodes/{node_id}", {
    params: { path: { node_id: id } },
    headers: bearer(credential),
  });
}
