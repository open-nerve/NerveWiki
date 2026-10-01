import type { AccountProfile, ApiClient, NotebookAuditEvent, OwnerlessNotebook } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer, displayNameOf } from "./auth";

// The ownerless notebooks and their audit events, through the API (M3/P3
// design 3.3, 3.4).

/** Rule two's refusal, with how many notebooks block in each workspace, by slug. */
export function soleAdminOfNotebooks(bySlug: Record<string, number>): string {
  const counts = Object.keys(bySlug)
    .toSorted()
    .map((slug) => `${bySlug[slug]} in ${slug}`);
  return `The account is the only admin of notebooks with other members (${counts.join(", ")}): another member must become their admin, or they must be deleted, first.`;
}

/** The profile of the account userId of email, as a workspace's admins see it. */
export function profileOf(userId: string, email: string): AccountProfile {
  return { user_id: userId, display_name: displayNameOf(email), email };
}

/** credential's list of the ownerless notebooks of the workspace of slug, as the API answers it. */
export async function listOwnerless(api: ApiClient, credential: string, slug: string) {
  return api.GET("/api/v0/workspaces/{slug}/ownerless-notebooks", {
    params: { path: { slug } },
    headers: bearer(credential),
  });
}

/** The ownerless notebooks of the workspace of slug, as credential, an admin's, lists them. */
export async function ownerlessNotebooks(
  api: ApiClient,
  credential: string,
  slug: string
): Promise<OwnerlessNotebook[]> {
  const { data, error, response } = await listOwnerless(api, credential, slug);
  expect(response.status, `list ${slug}'s ownerless notebooks: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** credential's take-over of the ownerless notebook id, as the API answers it. */
export async function takeOver(api: ApiClient, credential: string, id: string) {
  return api.POST("/api/v0/ownerless-notebooks/{notebook_id}/take-over", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}

/** credential's deletion of the ownerless notebook id, as the API answers it. */
export async function deleteOwnerless(api: ApiClient, credential: string, id: string) {
  return api.DELETE("/api/v0/ownerless-notebooks/{notebook_id}", {
    params: { path: { notebook_id: id } },
    headers: bearer(credential),
  });
}

/** credential's page of the audit events of the workspace of slug, as the API answers it. */
export async function listAuditEvents(
  api: ApiClient,
  credential: string,
  slug: string,
  query: { limit?: number; cursor?: string } = {}
) {
  return api.GET("/api/v0/workspaces/{slug}/notebook-audit-events", {
    params: { path: { slug }, query },
    headers: bearer(credential),
  });
}

/**
 * Every audit event of the workspace of slug from cursor on, newest first, as credential, an admin's, pages through
 * them, limit a page.
 */
export async function auditEvents(
  api: ApiClient,
  credential: string,
  slug: string,
  limit?: number,
  cursor?: string
): Promise<NotebookAuditEvent[]> {
  const { data, error, response } = await listAuditEvents(api, credential, slug, { limit, cursor });
  expect(response.status, `list ${slug}'s audit events: ${JSON.stringify(error)}`).toBe(200);
  const page = data?.data ?? [];
  return data?.next_cursor ? [...page, ...(await auditEvents(api, credential, slug, limit, data.next_cursor))] : page;
}

/** What a story checks of an audit event: action, notebook name, former owner's address, actor's address. */
export function eventOf(e: NotebookAuditEvent): [string, string, string, string] {
  return [e.action, e.notebook_name, e.former_owner.email, e.actor.email];
}
