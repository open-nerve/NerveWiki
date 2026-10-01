import type { ApiClient, Notebook, NotebookCreate, NotebookRole, NotebookUpdate } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { Notebook, NotebookCreate, NotebookRole, NotebookUpdate };

/**
 * NotebookService lists a workspace's notebooks the account sees, creates,
 * reads, changes and deletes them, and leaves them (M3/P4 design 3.2).
 */
export class NotebookService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the notebooks of the workspace of slug that the account sees, by name, each with its role there. */
  async list(slug: string): Promise<Notebook[]> {
    return (await unwrap(await this.api.GET("/api/v0/workspaces/{slug}/notebooks", { params: { path: { slug } } })))
      .data;
  }

  /** create answers the new notebook, of which the account is the admin. */
  async create(slug: string, body: NotebookCreate): Promise<Notebook> {
    return unwrap(await this.api.POST("/api/v0/workspaces/{slug}/notebooks", { params: { path: { slug } }, body }));
  }

  async get(id: string): Promise<Notebook> {
    return unwrap(await this.api.GET("/api/v0/notebooks/{notebook_id}", { params: { path: { notebook_id: id } } }));
  }

  async update(id: string, body: NotebookUpdate): Promise<Notebook> {
    return unwrap(
      await this.api.PATCH("/api/v0/notebooks/{notebook_id}", { params: { path: { notebook_id: id } }, body })
    );
  }

  async remove(id: string): Promise<void> {
    await unwrap(await this.api.DELETE("/api/v0/notebooks/{notebook_id}", { params: { path: { notebook_id: id } } }));
  }

  /** leave ends the account's membership of the notebook id. */
  async leave(id: string): Promise<void> {
    await unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/leave", { params: { path: { notebook_id: id } } })
    );
  }
}
