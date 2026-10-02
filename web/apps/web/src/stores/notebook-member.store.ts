import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { NotebookMember, NotebookMemberService } from "../services/notebook-member.service";
import type { NotebookRole } from "../services/notebook.service";
import { compare } from "./order";

/**
 * byJoined is the order of the server's list: when the membership was
 * created, then its id. A membership given back keeps when it was created,
 * so an addition is not always the last.
 */
function byJoined(a: NotebookMember, b: NotebookMember): number {
  return Date.parse(a.created_at) - Date.parse(b.created_at) || compare(a.id, b.id);
}

/**
 * NotebookMemberStore holds the active explicit members of one notebook,
 * in the server's order, for one generation (M3/P4 design 3.2).
 */
export class NotebookMemberStore {
  list: NotebookMember[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;
  /**
   * Each member's changes, one at a time (v0.1 design 13.2, item 1): the
   * store outlives the page, whose role menu, mounted anew, may send while
   * the change before is out (M3 Codex review R1).
   */
  private readonly inTurn = oneAtATimeById();

  constructor(
    private readonly service: Pick<NotebookMemberService, "list" | "add" | "update" | "remove">,
    /** The id of the notebook whose members these are. */
    private readonly notebookId: string
  ) {
    makeAutoObservable<this, "service" | "notebookId" | "changesAnswered" | "inTurn">(this, {
      service: false,
      notebookId: false,
      changesAnswered: false,
      inTurn: false,
    });
  }

  /**
   * load reads the list; SWR calls it. A change answered while the read was
   * out is newer than what it read: the list kept stays the one with it.
   */
  async load(): Promise<NotebookMember[]> {
    const answeredBefore = this.changesAnswered;
    const list = await this.service.list(this.notebookId);
    if (this.changesAnswered !== answeredBefore && this.list !== undefined) {
      return this.list;
    }
    runInAction(() => {
      this.list = list;
    });
    return list;
  }

  /** add makes the account userId a member with role, and answers the membership, a new one or one given back. */
  async add(userId: string, role: NotebookRole): Promise<NotebookMember> {
    const added = await this.service.add(this.notebookId, userId, role);
    // A read answered before the addition may hold it already.
    this.changed((list) => [...list.filter((member) => member.id !== added.id), added]);
    return added;
  }

  async changeRole(id: string, role: NotebookRole): Promise<void> {
    await this.inTurn(id, async () => {
      const changed = await this.service.update(id, role);
      this.changed((list) => list.map((member) => (member.id === id ? changed : member)));
    });
  }

  /** remove ends the membership id; one that has ended already (removed elsewhere, or left) is gone as well. */
  async remove(id: string): Promise<void> {
    await this.inTurn(id, async () => {
      try {
        await this.service.remove(id);
      } catch (error) {
        if (!(error instanceof ApiError && error.code === "notebook.member_not_found")) {
          throw error;
        }
      }
      this.changed((list) => list.filter((member) => member.id !== id));
    });
  }

  private changed(change: (list: NotebookMember[]) => NotebookMember[]): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.list !== undefined) {
        this.list = change(this.list).toSorted(byJoined);
      }
    });
  }
}
