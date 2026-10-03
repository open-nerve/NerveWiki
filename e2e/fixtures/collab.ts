import type { ApiClient, EditLock, EditSession } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The edit lock of the collaboration stories, through the API (M5 design
// 4.2): the opening that takes over, the lock's read and its release.

/** credential's opening of an edit session of the page id that takes their own over, as the API answers it. */
export async function postTakeOver(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/pages/{page_id}/edit-sessions", {
    params: { path: { page_id: id } },
    body: { take_over: true },
    headers: bearer(credential),
  });
}

/** Takes credential's sessions of the page id over, and returns the session opened. */
export async function takeOver(api: ApiClient, credential: string, id: string): Promise<EditSession> {
  const { data, error, response } = await postTakeOver(api, credential, id);
  expect(response.status, `take ${id} over: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`take ${id} over answered 201 without the session`);
  }
  return data;
}

/** credential's read of the page id's edit lock, as the API answers it. */
async function getLock(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/pages/{page_id}/edit-lock", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}

/** Reads the page id's edit lock with credential. */
export async function readLock(api: ApiClient, credential: string, id: string): Promise<EditLock> {
  const { data, error, response } = await getLock(api, credential, id);
  expect(response.status, `read the lock of ${id}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`read the lock of ${id} answered 200 without it`);
  }
  return data;
}

/** credential's release of the page id's edit lock, as the API answers it. */
export async function releaseLock(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/pages/{page_id}/edit-lock", {
    params: { path: { page_id: id } },
    headers: bearer(credential),
  });
}
