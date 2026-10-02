import type {
  ApiClient,
  EditSession,
  NodeMove,
  Page,
  PageContent,
  PageContentWrite,
  PageCreate,
  TreeNode,
} from "@nervewiki/api-client";
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

/**
 * Creates the page titled title in the notebook notebookId with credential, under parentId or at the root, with
 * content when given, and returns it.
 */
export async function createPage(
  api: ApiClient,
  credential: string,
  notebookId: string,
  title: string,
  parentId: string | null = null,
  content?: string
): Promise<Page> {
  const body: PageCreate =
    content === undefined ? { parent_id: parentId, title } : { parent_id: parentId, title, content };
  const { data, error, response } = await postPage(api, credential, notebookId, body);
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

/** credential's read of the page id's content, as the API answers it. */
export async function getContent(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/pages/{page_id}/content", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}

/** The page id's content as credential reads it. */
export async function readContent(api: ApiClient, credential: string, id: string): Promise<PageContent> {
  const { data, error, response } = await getContent(api, credential, id);
  expect(response.status, `read the content of ${id}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`read the content of ${id} answered 200 without it`);
  }
  return data;
}

/** credential's write of body to the page id's content, as the API answers it. */
export async function putContent(api: ApiClient, credential: string, id: string, body: PageContentWrite) {
  return api.PUT("/api/v0/pages/{page_id}/content", {
    params: { path: { page_id: id } },
    body,
    headers: bearer(credential),
  });
}

/** Writes body to the page id's content with credential, and returns the page as the API answers it. */
export async function writeContent(
  api: ApiClient,
  credential: string,
  id: string,
  body: PageContentWrite
): Promise<Page> {
  const { data, error, response } = await putContent(api, credential, id, body);
  expect(response.status, `write the content of ${id}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`write the content of ${id} answered 200 without the page`);
  }
  return data;
}

/** credential's read of the page id's reading view, as the API answers it. */
export async function getView(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/pages/{page_id}/view", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}

/** credential's opening of an edit session of the page id, as the API answers it. */
export async function postSession(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/pages/{page_id}/edit-sessions", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}

/** Opens an edit session of the page id with credential, and returns it. */
export async function openSession(api: ApiClient, credential: string, id: string): Promise<EditSession> {
  const { data, error, response } = await postSession(api, credential, id);
  expect(response.status, `open a session of ${id}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`open a session of ${id} answered 201 without it`);
  }
  return data;
}

/** credential's heartbeat of the edit session id, as the API answers it. */
export async function heartbeat(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/edit-sessions/{edit_session_id}/heartbeat", {
    params: { path: { edit_session_id: id } },
    headers: bearer(credential),
  });
}

/** credential's end of the edit session id, as the API answers it. */
export async function endSession(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/edit-sessions/{edit_session_id}", {
    params: { path: { edit_session_id: id } },
    headers: bearer(credential),
  });
}
