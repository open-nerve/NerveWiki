import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATime } from "../lib/one-at-a-time";
import type { AccountService, UpdateMeRequest, User } from "../services/account.service";

/**
 * AccountStore holds the signed-in account of one generation. Its changes
 * go out one at a time: each answers the whole account, and the last one
 * answered is the account the server holds (M1/P5 design 3.3).
 */
export class AccountStore {
  me: User | undefined = undefined;
  private readonly changes = oneAtATime();

  constructor(private readonly service: Pick<AccountService, "getMe" | "updateMe" | "recordStep">) {
    makeAutoObservable<this, "service" | "changes">(this, { service: false, changes: false });
  }

  /** load reads the account; SWR calls it (useSWR with this as fetcher). */
  async load(): Promise<User> {
    return this.keep(await this.service.getMe());
  }

  update(changes: UpdateMeRequest): Promise<User> {
    return this.changes(async () => this.keep(await this.service.updateMe(changes)));
  }

  recordStep(step: string): Promise<User> {
    return this.changes(async () => this.keep(await this.service.recordStep(step)));
  }

  private keep(me: User): User {
    runInAction(() => {
      this.me = me;
    });
    return me;
  }
}
