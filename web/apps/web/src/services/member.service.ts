import type { ApiClient, WorkspaceMember, WorkspaceRole } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { WorkspaceMember, WorkspaceRole };

/** MemberService lists a workspace's members, changes their roles and removes them (M2/P6 design 3.2). */
export class MemberService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the active members of the workspace of slug, by when they joined; a guest sees no addresses. */
  async list(slug: string): Promise<WorkspaceMember[]> {
    return (await unwrap(await this.api.GET("/api/v0/workspaces/{slug}/members", { params: { path: { slug } } }))).data;
  }

  async update(id: string, role: WorkspaceRole): Promise<WorkspaceMember> {
    return unwrap(
      await this.api.PATCH("/api/v0/workspace-members/{workspace_member_id}", {
        params: { path: { workspace_member_id: id } },
        body: { role },
      })
    );
  }

  async remove(id: string): Promise<void> {
    await unwrap(
      await this.api.DELETE("/api/v0/workspace-members/{workspace_member_id}", {
        params: { path: { workspace_member_id: id } },
      })
    );
  }
}
