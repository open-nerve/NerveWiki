import type { ApiClient, NotebookMember, NotebookRole } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The notebook members of the stories, through the API (M3/P2 design 3.2).

/** The members of the notebook id that credential lists. */
export async function listNotebookMembers(api: ApiClient, credential: string, id: string): Promise<NotebookMember[]> {
  const { data, error, response } = await api.GET("/api/v0/notebooks/{notebook_id}/members", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
  expect(response.status, `list ${id}'s members: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** credential's addition of the account userId to the notebook id as role, as the API answers it. */
export async function addNotebookMember(
  api: ApiClient,
  credential: string,
  id: string,
  userId: string,
  role: NotebookRole
) {
  return api.POST("/api/v0/notebooks/{notebook_id}/members", {
    params: { path: { notebook_id: id } },
    body: { user_id: userId, role },
    headers: bearer(credential),
  });
}

/** credential's change of the membership id to role, as the API answers it. */
export async function updateNotebookMember(api: ApiClient, credential: string, id: string, role: NotebookRole) {
  return api.PATCH("/api/v0/notebook-members/{notebook_member_id}", {
    params: { path: { notebook_member_id: id } },
    body: { role },
    headers: bearer(credential),
  });
}

/** credential's removal of the membership id, as the API answers it. */
export async function removeNotebookMember(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/notebook-members/{notebook_member_id}", {
    params: { path: { notebook_member_id: id } },
    headers: bearer(credential),
  });
}

/** credential leaving the notebook id, as the API answers it. */
export async function leaveNotebook(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/notebooks/{notebook_id}/leave", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}

/** Adds userId to the notebook id as role with credential, and returns the membership. */
export async function addedNotebookMember(
  api: ApiClient,
  credential: string,
  id: string,
  userId: string,
  role: NotebookRole
): Promise<NotebookMember> {
  const { data, error, response } = await addNotebookMember(api, credential, id, userId, role);
  expect(response.status, `add ${userId} to ${id}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`add ${userId} answered 201 without the member`);
  }
  return data;
}
