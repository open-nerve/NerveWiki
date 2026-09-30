import type { ApiClient, UpdateMeRequest, User } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { UpdateMeRequest, User };

/** AccountService reads and changes the signed-in account, on its session's client. */
export class AccountService {
  constructor(private readonly api: ApiClient) {}

  async getMe(): Promise<User> {
    return unwrap(await this.api.GET("/api/v0/me"));
  }

  async updateMe(changes: UpdateMeRequest): Promise<User> {
    return unwrap(await this.api.PATCH("/api/v0/me", { body: changes }));
  }

  /**
   * changePassword replaces the password once current is confirmed: the
   * account's other sessions end, this one goes on (204).
   */
  async changePassword(current: string, next: string): Promise<void> {
    await unwrap(
      await this.api.POST("/api/v0/me/change-password", { body: { current_password: current, new_password: next } })
    );
  }

  /** deactivate deactivates the account: every session ends, this one too (204). */
  async deactivate(): Promise<void> {
    await unwrap(await this.api.POST("/api/v0/me/deactivate"));
  }

  /** recordStep records a completed onboarding step; recording it again changes nothing. */
  async recordStep(step: string): Promise<User> {
    return unwrap(await this.api.POST("/api/v0/me/onboarding-steps", { body: { step } }));
  }
}
