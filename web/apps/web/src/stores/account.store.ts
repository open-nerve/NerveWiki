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
  /** How many changes have been answered: a read that overlaps one may have read the account before it. */
  private changesAnswered = 0;

  constructor(private readonly service: Pick<AccountService, "getMe" | "updateMe" | "recordStep">) {
    makeAutoObservable<this, "service" | "changes" | "changesAnswered">(this, {
      service: false,
      changes: false,
      changesAnswered: false,
    });
  }

  /**
   * load reads the account; SWR calls it (useSWR with this as fetcher), on
   * focus too. A change answered while the read was out is newer than what
   * it read: the account kept stays that change's answer.
   */
  async load(): Promise<User> {
    const answeredBefore = this.changesAnswered;
    const me = await this.service.getMe();
    if (this.changesAnswered !== answeredBefore && this.me !== undefined) {
      return this.me;
    }
    return this.keep(me);
  }

  update(changes: UpdateMeRequest): Promise<User> {
    return this.changes(async () => this.keepChange(await this.service.updateMe(changes)));
  }

  recordStep(step: string): Promise<User> {
    return this.changes(async () => this.keepChange(await this.service.recordStep(step)));
  }

  private keepChange(me: User): User {
    this.changesAnswered += 1;
    return this.keep(me);
  }

  private keep(me: User): User {
    runInAction(() => {
      this.me = me;
    });
    return me;
  }
}
