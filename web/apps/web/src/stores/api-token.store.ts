import { makeAutoObservable, runInAction } from "mobx";

import { ApiError } from "../services/api";
import type { ApiToken, ApiTokenCreate, ApiTokenCreated, ApiTokenService } from "../services/api-token.service";

/**
 * ApiTokenStore holds the personal access tokens of one generation's
 * account, as the list shows them: never a token itself, which only the
 * creating dialog holds, while it is open (M1/P6 design 3.6).
 */
export class ApiTokenStore {
  tokens: ApiToken[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;

  constructor(private readonly service: Pick<ApiTokenService, "list" | "create" | "revoke">) {
    makeAutoObservable<this, "service" | "changesAnswered">(this, { service: false, changesAnswered: false });
  }

  /**
   * load reads the list; SWR calls it (useSWR with this as fetcher). A
   * change answered while the read was out is newer than what it read: the
   * list kept stays the one with the change.
   */
  async load(): Promise<ApiToken[]> {
    const answeredBefore = this.changesAnswered;
    const tokens = await this.service.list();
    if (this.changesAnswered !== answeredBefore && this.tokens !== undefined) {
      return this.tokens;
    }
    runInAction(() => {
      this.tokens = tokens;
    });
    return tokens;
  }

  /** create answers the new token with its secret, for the caller to show once; the list gets it without. */
  async create(body: ApiTokenCreate): Promise<ApiTokenCreated> {
    const created = await this.service.create(body);
    const { id, name, expires_at, last_used_at, created_at } = created;
    this.changed((tokens) => [{ id, name, expires_at, last_used_at, created_at }, ...tokens]);
    return created;
  }

  /** revoke revokes the token; one that is gone already (revoked in another tab) is revoked as well. */
  async revoke(id: string): Promise<void> {
    try {
      await this.service.revoke(id);
    } catch (error) {
      if (!(error instanceof ApiError && error.code === "identity.api_token_not_found")) {
        throw error;
      }
    }
    this.changed((tokens) => tokens.filter((token) => token.id !== id));
  }

  private changed(change: (tokens: ApiToken[]) => ApiToken[]): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.tokens !== undefined) {
        this.tokens = change(this.tokens);
      }
    });
  }
}
