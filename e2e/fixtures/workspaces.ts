import { createHash } from "node:crypto";

import type { ApiClient, Workspace } from "@nervewiki/api-client";
import { expect, type TestInfo } from "@playwright/test";

import { bearer } from "./auth";

// The workspaces of the stories, through the API (M2 design 3).

/**
 * A slug of this run of this test: the tests of a worker share its
 * database, and --repeat-each runs a test again in the same worker.
 */
export function slugFor(testInfo: TestInfo, label = "ws"): string {
  const run = `${testInfo.testId}-${testInfo.repeatEachIndex}-${testInfo.retry}`;
  return `${label}-${createHash("sha256").update(run).digest("hex").slice(0, 12)}`;
}

/** Creates the workspace name and slug with credential, as its admin, and returns it. */
export async function createWorkspace(
  api: ApiClient,
  credential: string,
  name: string,
  slug: string
): Promise<Workspace> {
  const { data, error, response } = await api.POST("/api/v0/workspaces", {
    body: { name, slug },
    headers: bearer(credential),
  });
  expect(response.status, `create ${slug}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`create ${slug} answered 201 without the workspace`);
  }
  return data;
}

/** What checkWorkspaceSlug answers of slug. */
export async function checkSlug(api: ApiClient, credential: string, slug: string): Promise<unknown> {
  const { data, response } = await api.GET("/api/v0/workspace-slugs/{slug}", {
    params: { path: { slug } },
    headers: bearer(credential),
  });
  expect(response.status).toBe(200);
  return data;
}

/** Renames the workspace of slug with credential, as its admin, and returns it. */
export async function renameWorkspace(
  api: ApiClient,
  credential: string,
  slug: string,
  name: string
): Promise<Workspace> {
  const { data, error, response } = await api.PATCH("/api/v0/workspaces/{slug}", {
    params: { path: { slug } },
    body: { name },
    headers: bearer(credential),
  });
  expect(response.status, `rename ${slug}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`rename ${slug} answered 200 without the workspace`);
  }
  return data;
}

/** Deletes the workspace of slug with credential, as its admin. */
export async function deleteWorkspace(api: ApiClient, credential: string, slug: string): Promise<void> {
  const { error, response } = await api.DELETE("/api/v0/workspaces/{slug}", {
    params: { path: { slug } },
    headers: bearer(credential),
  });
  expect(response.status, `delete ${slug}: ${JSON.stringify(error)}`).toBe(204);
}
