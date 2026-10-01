import type { ApiClient, WorkspaceMember } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";

// The members of the stories' workspaces, through the API (M2/P2 design 3.2).

/** The active members of the workspace of slug, as credential sees them. */
export async function listMembers(api: ApiClient, credential: string, slug: string): Promise<WorkspaceMember[]> {
  const { data, error, response } = await api.GET("/api/v0/workspaces/{slug}/members", {
    params: { path: { slug } },
    headers: bearer(credential),
  });
  expect(response.status, `list ${slug}'s members: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** The membership of the account of email in the workspace of slug, as credential, an admin's, lists it. */
export async function memberOf(
  api: ApiClient,
  credential: string,
  slug: string,
  email: string
): Promise<WorkspaceMember> {
  const member = (await listMembers(api, credential, slug)).find((m) => m.email === email);
  if (!member) {
    throw new Error(`${email} is no member of ${slug}`);
  }
  return member;
}

/** credential's change of the membership id's role, as the API answers it. */
export async function updateMember(api: ApiClient, credential: string, id: string, role: WorkspaceMember["role"]) {
  return api.PATCH("/api/v0/workspace-members/{workspace_member_id}", {
    params: { path: { workspace_member_id: id } },
    body: { role },
    headers: bearer(credential),
  });
}

/** credential's removal of the membership id, as the API answers it. */
export async function removeMember(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/workspace-members/{workspace_member_id}", {
    params: { path: { workspace_member_id: id } },
    headers: bearer(credential),
  });
}

/** credential's leaving of the workspace of slug, as the API answers it. */
export async function leave(api: ApiClient, credential: string, slug: string) {
  return api.POST("/api/v0/workspaces/{slug}/leave", { params: { path: { slug } }, headers: bearer(credential) });
}
