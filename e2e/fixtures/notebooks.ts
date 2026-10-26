import type {
  ApiClient,
  Notebook,
  NotebookCreate,
  NotebookMode,
  NotebookUpdate,
  WorkspaceAccess,
} from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The notebooks of the stories, through the API (M3 design 5).

/** What a notebook is created as besides its name and workspace access: its mode and its template's language. */
interface NotebookOptions {
  mode?: NotebookMode;
  templateLocale?: "zh-CN" | "en";
}

/**
 * Creates the notebook name in the workspace of slug with credential, as its admin, and returns it; the mode and the
 * template's language are sent when options give them (M10/P1 design 5.1).
 */
export async function createNotebook(
  api: ApiClient,
  credential: string,
  slug: string,
  name: string,
  workspaceAccess?: WorkspaceAccess,
  options: NotebookOptions = {}
): Promise<Notebook> {
  const body: NotebookCreate = { name };
  if (workspaceAccess !== undefined) {
    body.workspace_access = workspaceAccess;
  }
  if (options.mode !== undefined) {
    body.mode = options.mode;
  }
  if (options.templateLocale !== undefined) {
    body.template_locale = options.templateLocale;
  }
  const { data, error, response } = await api.POST("/api/v0/workspaces/{slug}/notebooks", {
    params: { path: { slug } },
    body,
    headers: bearer(credential),
  });
  expect(response.status, `create ${name} in ${slug}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`create ${name} answered 201 without the notebook`);
  }
  return data;
}

/** The notebooks of the workspace of slug that credential sees. */
export async function listNotebooks(api: ApiClient, credential: string, slug: string): Promise<Notebook[]> {
  const { data, error, response } = await api.GET("/api/v0/workspaces/{slug}/notebooks", {
    params: { path: { slug } },
    headers: bearer(credential),
  });
  expect(response.status, `list ${slug}'s notebooks: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** credential's read of the notebook id, as the API answers it. */
export async function getNotebook(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}

/** credential's change of the notebook id, as the API answers it. */
export async function updateNotebook(api: ApiClient, credential: string, id: string, body: NotebookUpdate) {
  return api.PATCH("/api/v0/notebooks/{notebook_id}", {
    params: { path: { notebook_id: id } },
    body,
    headers: bearer(credential),
  });
}

/** credential's downgrade of the notebook id, as the API answers it, including a refusal. */
export async function downgradeNotebook(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/notebooks/{notebook_id}/downgrade", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}

/** credential's deletion of the notebook id, as the API answers it. */
export async function deleteNotebook(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/notebooks/{notebook_id}", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}
