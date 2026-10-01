import type { ApiClient, NotebookMember, NotebookRole } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { NotebookMember };

/** NotebookMemberService lists a notebook's members, adds them, changes their roles and removes them (M3/P4 design 3.2). */
export class NotebookMemberService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the active explicit members of the notebook id, by when they joined; a guest sees no addresses. */
  async list(notebookId: string): Promise<NotebookMember[]> {
    return (
      await unwrap(
        await this.api.GET("/api/v0/notebooks/{notebook_id}/members", { params: { path: { notebook_id: notebookId } } })
      )
    ).data;
  }

  /** add makes the account userId, an active member of the notebook's workspace, a member of it with role. */
  async add(notebookId: string, userId: string, role: NotebookRole): Promise<NotebookMember> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/members", {
        params: { path: { notebook_id: notebookId } },
        body: { user_id: userId, role },
      })
    );
  }

  async update(id: string, role: NotebookRole): Promise<NotebookMember> {
    return unwrap(
      await this.api.PATCH("/api/v0/notebook-members/{notebook_member_id}", {
        params: { path: { notebook_member_id: id } },
        body: { role },
      })
    );
  }

  async remove(id: string): Promise<void> {
    await unwrap(
      await this.api.DELETE("/api/v0/notebook-members/{notebook_member_id}", {
        params: { path: { notebook_member_id: id } },
      })
    );
  }
}
