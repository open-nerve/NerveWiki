import type {
  ApiClient,
  Notebook,
  NotebookAuditEvent,
  NotebookAuditEventPage,
  OwnerlessNotebook,
} from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { NotebookAuditEvent, NotebookAuditEventPage, OwnerlessNotebook };

/**
 * OwnerlessService lists a workspace's ownerless notebooks to its admins,
 * takes them over and deletes them, and reads the audit events of both and
 * of the returns (M3/P5 design 3.2).
 */
export class OwnerlessService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the workspace's ownerless notebooks, the longest ownerless first. */
  async list(slug: string): Promise<OwnerlessNotebook[]> {
    return (
      await unwrap(await this.api.GET("/api/v0/workspaces/{slug}/ownerless-notebooks", { params: { path: { slug } } }))
    ).data;
  }

  /** takeOver makes the account the notebook's admin, and answers the notebook as the account now sees it. */
  async takeOver(id: string): Promise<Notebook> {
    return unwrap(
      await this.api.POST("/api/v0/ownerless-notebooks/{notebook_id}/take-over", {
        params: { path: { notebook_id: id } },
      })
    );
  }

  async remove(id: string): Promise<void> {
    await unwrap(
      await this.api.DELETE("/api/v0/ownerless-notebooks/{notebook_id}", { params: { path: { notebook_id: id } } })
    );
  }

  /** auditEvents answers a page of the workspace's audit events, the newest first: the first, or the one cursor names. */
  async auditEvents(slug: string, cursor?: string): Promise<NotebookAuditEventPage> {
    return unwrap(
      await this.api.GET("/api/v0/workspaces/{slug}/notebook-audit-events", {
        params: { path: { slug }, query: cursor === undefined ? {} : { cursor } },
      })
    );
  }
}
