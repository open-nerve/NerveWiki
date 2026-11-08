import type { ApiClient, Upgrade, UpgradeStart } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// A notebook's upgrade to an LLM Wiki in the stories, through the API
// (M11/P1 design 5): its start and its read.

/** credential's start of the upgrade of the notebook id, as the API answers it, including a refusal. */
export async function postUpgrade(api: ApiClient, credential: string, id: string, body: UpgradeStart = {}) {
  return api.POST("/api/v0/notebooks/{notebook_id}/upgrade", {
    params: { path: { notebook_id: id } },
    body,
    headers: bearer(credential),
  });
}

/** Starts the upgrade of the notebook id with credential, and returns it. */
export async function startUpgrade(api: ApiClient, credential: string, id: string): Promise<Upgrade> {
  const { data, error, response } = await postUpgrade(api, credential, id);
  expect(response.status, `start the upgrade of ${id}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`start the upgrade of ${id} answered 200 without it`);
  }
  return data;
}

/** credential's read of the upgrade under way of the notebook id, as the API answers it. */
export async function getUpgrade(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/notebooks/{notebook_id}/upgrade", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}
