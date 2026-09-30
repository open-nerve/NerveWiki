import { makeAutoObservable, runInAction } from "mobx";

import type { InstanceInfo, InstanceService } from "../services/instance.service";

/** InstanceStore holds what this instance runs, once loaded. */
export class InstanceStore {
  info: InstanceInfo | undefined = undefined;

  constructor(private readonly service: Pick<InstanceService, "get">) {
    makeAutoObservable<this, "service">(this, { service: false });
  }

  /** fetch loads the information; SWR calls it (useSWR with this as fetcher). */
  async fetch(): Promise<InstanceInfo> {
    const info = await this.service.get();
    runInAction(() => {
      this.info = info;
    });
    return info;
  }
}
