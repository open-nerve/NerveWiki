import type {
  ApiClient,
  AuthTokens,
  InvitationPreview,
  Workspace,
  WorkspaceInvitation,
  WorkspaceRole,
} from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer, createToken, register, registerOnboarded } from "./auth";

// The invitations of the stories, through the API (M2/P3 design 3.3). Each
// helper expects success, but preview and tryAccept, which return the
// answer; a story asks for a refusal itself.

/** Invites email to the workspace of slug as role with credential, an admin's, and returns the invitation. */
export async function invite(
  api: ApiClient,
  credential: string,
  slug: string,
  email: string,
  role: WorkspaceRole = "member"
): Promise<WorkspaceInvitation> {
  const { data, error, response } = await api.POST("/api/v0/workspaces/{slug}/invitations", {
    params: { path: { slug } },
    body: { email, role },
    headers: bearer(credential),
  });
  expect(response.status, `invite ${email} to ${slug}: ${JSON.stringify(error)}`).toBe(201);
  if (!data) {
    throw new Error(`invite ${email} answered 201 without the invitation`);
  }
  return data;
}

/** The pending invitations of the workspace of slug, as credential, an admin's, lists them. */
export async function listInvitations(
  api: ApiClient,
  credential: string,
  slug: string
): Promise<WorkspaceInvitation[]> {
  const { data, error, response } = await api.GET("/api/v0/workspaces/{slug}/invitations", {
    params: { path: { slug } },
    headers: bearer(credential),
  });
  expect(response.status, `list ${slug}'s invitations: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** Withdraws the invitation id with credential, an admin's. */
export async function withdraw(api: ApiClient, credential: string, id: string): Promise<void> {
  const { error, response } = await api.DELETE("/api/v0/workspace-invitations/{workspace_invitation_id}", {
    params: { path: { workspace_invitation_id: id } },
    headers: bearer(credential),
  });
  expect(response.status, `withdraw ${id}: ${JSON.stringify(error)}`).toBe(204);
}

/** What the invitation's link shows anyone, with no credential. */
export async function preview(api: ApiClient, invitation: Pick<WorkspaceInvitation, "id" | "token">) {
  return api.POST("/api/v0/workspace-invitations/{workspace_invitation_id}/preview", {
    params: { path: { workspace_invitation_id: invitation.id } },
    body: { token: invitation.token },
  });
}

/** credential's acceptance of the invitation, as the API answers it. */
export async function tryAccept(
  api: ApiClient,
  credential: string,
  invitation: Pick<WorkspaceInvitation, "id" | "token">
) {
  return api.POST("/api/v0/workspace-invitations/{workspace_invitation_id}/accept", {
    params: { path: { workspace_invitation_id: invitation.id } },
    body: { token: invitation.token },
    headers: bearer(credential),
  });
}

/** Accepts the invitation with credential and returns the workspace, with the caller's role. */
export async function accept(
  api: ApiClient,
  credential: string,
  invitation: Pick<WorkspaceInvitation, "id" | "token">
): Promise<Workspace> {
  const { data, error, response } = await tryAccept(api, credential, invitation);
  expect(response.status, `accept ${invitation.id}: ${JSON.stringify(error)}`).toBe(200);
  if (!data) {
    throw new Error(`accept ${invitation.id} answered 200 without the workspace`);
  }
  return data;
}

/** What preview answers to a link of a workspace named name, with slug, for role. */
export function previewOf(name: string, slug: string, role: WorkspaceRole): InvitationPreview {
  return { workspace: { name, slug }, role };
}

/**
 * Registers email, which joins the workspace of slug as role by an
 * invitation of adminCredential, and returns a personal access token of
 * it: a token outlives an administrator's change of its address, which
 * ends its sessions.
 */
export async function joinAs(
  api: ApiClient,
  adminCredential: string,
  slug: string,
  email: string,
  role: WorkspaceRole
): Promise<string> {
  const invitation = await invite(api, adminCredential, slug, email, role);
  const session = await register(api, email);
  await accept(api, session.access_token, invitation);
  return (await createToken(api, session.access_token, { name: `${role} of ${slug}` })).token;
}

/**
 * Registers email, onboarded, which joins the workspace of slug as role by
 * an invitation of adminCredential, and returns its tokens: for a page
 * signed in as it.
 */
export async function joinOnboarded(
  api: ApiClient,
  adminCredential: string,
  slug: string,
  email: string,
  role: WorkspaceRole
): Promise<AuthTokens> {
  const tokens = await registerOnboarded(api, email);
  await accept(api, tokens.access_token, await invite(api, adminCredential, slug, email, role));
  return tokens;
}
