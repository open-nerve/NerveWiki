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

  /** recordStep records a completed onboarding step; recording it again changes nothing. */
  async recordStep(step: string): Promise<User> {
    return unwrap(await this.api.POST("/api/v0/me/onboarding-steps", { body: { step } }));
  }
}
