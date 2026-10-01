import { createHash } from "node:crypto";

import type { ApiClient, AuthTokens, Workspace } from "@nervewiki/api-client";
import { expect, type TestInfo } from "@playwright/test";

import { bearer, createToken, emailFor, register, registerOnboarded } from "./auth";

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

/** A story's workspace: its admin's address, account and personal access token, and the workspace. */
export interface Team {
  adminEmail: string;
  adminId: string;
  pat: string;
  workspace: Workspace;
}

/** Registers an admin of this test, who creates a workspace named name with a slug of this test. */
export async function newTeam(api: ApiClient, testInfo: TestInfo, name = "Acme"): Promise<Team> {
  return teamOf(api, testInfo, name, await register(api, emailFor(testInfo, "admin")));
}

/** newTeam's, its admin onboarded, with the admin's tokens: for a page signed in as the admin. */
export async function newOnboardedTeam(
  api: ApiClient,
  testInfo: TestInfo,
  name = "Acme"
): Promise<Team & { tokens: AuthTokens }> {
  const tokens = await registerOnboarded(api, emailFor(testInfo, "admin"));
  return { ...(await teamOf(api, testInfo, name, tokens)), tokens };
}

/** The team of the admin of this test signed in with session, who creates a workspace named name. */
async function teamOf(api: ApiClient, testInfo: TestInfo, name: string, session: AuthTokens): Promise<Team> {
  const adminEmail = emailFor(testInfo, "admin");
  const pat = (await createToken(api, session.access_token, { name: testInfo.title.slice(0, 40) })).token;
  const me = await api.GET("/api/v0/me", { headers: bearer(pat) });
  if (!me.data) {
    throw new Error(`me answered ${me.response.status}`);
  }
  return { adminEmail, adminId: me.data.id, pat, workspace: await createWorkspace(api, pat, name, slugFor(testInfo)) };
}
