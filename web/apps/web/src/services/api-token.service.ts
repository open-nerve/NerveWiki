import type { ApiClient, ApiToken, ApiTokenCreate, ApiTokenCreated } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { ApiToken, ApiTokenCreate, ApiTokenCreated };

/** ApiTokenService lists, creates and revokes the signed-in account's personal access tokens. */
export class ApiTokenService {
  constructor(private readonly api: ApiClient) {}

  /** list answers the tokens that are not revoked, expired ones too, newest first; never the tokens themselves. */
  async list(): Promise<ApiToken[]> {
    return (await unwrap(await this.api.GET("/api/v0/me/api-tokens"))).data;
  }

  /** create answers the new token with the token itself, this once; it asks for the current password. */
  async create(body: ApiTokenCreate): Promise<ApiTokenCreated> {
    return unwrap(await this.api.POST("/api/v0/me/api-tokens", { body }));
  }

  async revoke(id: string): Promise<void> {
    await unwrap(await this.api.DELETE("/api/v0/api-tokens/{token_id}", { params: { path: { token_id: id } } }));
  }
}
