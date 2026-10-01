import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { Notebook } from "../services/notebook.service";
import type { OwnerlessNotebook, OwnerlessService } from "../services/ownerless.service";

/**
 * OwnerlessStore holds a workspace's ownerless notebooks, as its admins see
 * them, in the server's order, for one generation (M3/P5 design 3.2).
 * Taking one over or deleting it takes it off the list; so does a refusal
 * that says it is ownerless no more (taken over, deleted or returned since),
 * which the store still throws: what was asked did not happen.
 */
export class OwnerlessStore {
  list: OwnerlessNotebook[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;
  /** Each notebook's changes, one at a time (v0.1 design 13.2, item 1). */
  private readonly inTurn = oneAtATimeById();

  constructor(
    private readonly service: Pick<OwnerlessService, "list" | "takeOver" | "remove">,
    /** The slug of the workspace whose ownerless notebooks these are. */
    private readonly slug: string
  ) {
    makeAutoObservable<this, "service" | "slug" | "changesAnswered" | "inTurn">(this, {
      service: false,
      slug: false,
      changesAnswered: false,
      inTurn: false,
    });
  }

  /**
   * load reads the list; SWR calls it. A change answered while the read was
   * out is newer than what it read: the list kept stays the one with it,
   * or, before the first list, the list is read again.
   */
  async load(): Promise<OwnerlessNotebook[]> {
    const answeredBefore = this.changesAnswered;
    const list = await this.service.list(this.slug);
    if (this.changesAnswered !== answeredBefore) {
      return this.list ?? this.load();
    }
    runInAction(() => {
      this.list = list;
    });
    return list;
  }

  /** takeOver makes the account the admin of the notebook id, and answers it as the account now sees it. */
  async takeOver(id: string): Promise<Notebook> {
    return this.inTurn(id, () => this.settle(id, () => this.service.takeOver(id)));
  }

  async remove(id: string): Promise<void> {
    await this.inTurn(id, () => this.settle(id, () => this.service.remove(id)));
  }

  /** settle runs change of the notebook id, which then is off the list; a 404 takes it off too, and is thrown. */
  private async settle<T>(id: string, change: () => Promise<T>): Promise<T> {
    try {
      const answer = await change();
      this.gone(id);
      return answer;
    } catch (error) {
      if (error instanceof ApiError && error.code === "notebook.not_found") {
        this.gone(id);
      }
      throw error;
    }
  }

  private gone(id: string): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.list !== undefined) {
        this.list = this.list.filter((notebook) => notebook.id !== id);
      }
    });
  }
}
