import type { ApiClient, SlugAvailability, Workspace, WorkspaceCreate } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { SlugAvailability, Workspace, WorkspaceCreate };

/** WorkspaceService lists, creates, renames and deletes the signed-in account's workspaces (M2/P5 design 3.4). */
export class WorkspaceService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the workspaces of the account's active memberships, by name, each with its role. */
  async list(): Promise<Workspace[]> {
    return (await unwrap(await this.api.GET("/api/v0/workspaces"))).data;
  }

  /** create answers the new workspace, of which the account is the admin. */
  async create(body: WorkspaceCreate): Promise<Workspace> {
    return unwrap(await this.api.POST("/api/v0/workspaces", { body }));
  }

  async rename(slug: string, name: string): Promise<Workspace> {
    return unwrap(await this.api.PATCH("/api/v0/workspaces/{slug}", { params: { path: { slug } }, body: { name } }));
  }

  async remove(slug: string): Promise<void> {
    await unwrap(await this.api.DELETE("/api/v0/workspaces/{slug}", { params: { path: { slug } } }));
  }

  /** checkSlug answers whether slug can name a new workspace, and why not. */
  async checkSlug(slug: string): Promise<SlugAvailability> {
    return unwrap(await this.api.GET("/api/v0/workspace-slugs/{slug}", { params: { path: { slug } } }));
  }
}
