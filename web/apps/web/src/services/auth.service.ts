import type { ApiClient, AuthTokens } from "@nervewiki/api-client";

import { unwrap } from "./api";
import type { InvitationLink } from "./invitation.service";

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

  /**
   * register creates an account; with an invitation, the address it was
   * sent to registers while sign-up is closed (M2 design 4). It does not
   * accept the invitation.
   */
  async register(email: string, password: string, invitation?: InvitationLink): Promise<AuthTokens> {
    return unwrap(await this.api.POST("/api/v0/auth/register", { body: { email, password, invitation } }));
  }
}
