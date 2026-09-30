import type { ApiClient, InstanceInfo } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { InstanceInfo };

/** InstanceService asks the API what this instance runs. */
export class InstanceService {
  constructor(private readonly api: ApiClient) {}

  async get(): Promise<InstanceInfo> {
    return unwrap(await this.api.GET("/api/v0/instance"));
  }
}
