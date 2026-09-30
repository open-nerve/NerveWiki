import type { ApiClient, AuthTokens } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { AuthTokens };

/**
 * AuthService signs in and signs up, on the session's public client: a
 * wrong password answers 401, which must not look like the end of a
 * session (M1/P5 design 3.2).
 */
export class AuthService {
  constructor(private readonly api: ApiClient) {}

  async login(email: string, password: string): Promise<AuthTokens> {
    return unwrap(await this.api.POST("/api/v0/auth/login", { body: { email, password } }));
  }

  async register(email: string, password: string): Promise<AuthTokens> {
    return unwrap(await this.api.POST("/api/v0/auth/register", { body: { email, password } }));
  }
}
