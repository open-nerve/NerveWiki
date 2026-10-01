import type { ApiClient, WorkspaceInvitation, WorkspaceInvitationCreate } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { WorkspaceInvitation, WorkspaceInvitationCreate };

/**
 * InvitationService lists a workspace's pending invitations, with their
 * links' tokens, creates and withdraws them: an admin's (M2/P6 design 3.2).
 */
export class InvitationService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the pending invitations of the workspace of slug, newest first. */
  async list(slug: string): Promise<WorkspaceInvitation[]> {
    return (await unwrap(await this.api.GET("/api/v0/workspaces/{slug}/invitations", { params: { path: { slug } } })))
      .data;
  }

  async create(slug: string, body: WorkspaceInvitationCreate): Promise<WorkspaceInvitation> {
    return unwrap(await this.api.POST("/api/v0/workspaces/{slug}/invitations", { params: { path: { slug } }, body }));
  }

  async remove(id: string): Promise<void> {
    await unwrap(
      await this.api.DELETE("/api/v0/workspace-invitations/{workspace_invitation_id}", {
        params: { path: { workspace_invitation_id: id } },
      })
    );
  }
}
