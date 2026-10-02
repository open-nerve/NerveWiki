import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { MemberService, WorkspaceMember, WorkspaceRole } from "../services/member.service";

/**
 * MemberStore holds the members of one workspace, in the server's order
 * (by when they joined), for one generation (M2/P6 design 3.2).
 */
export class MemberStore {
  list: WorkspaceMember[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;
  /**
   * Each member's changes, one at a time (v0.1 design 13.2, item 1): the
   * store outlives the page, whose role menu, mounted anew, may send while
   * the change before is out (M3 Codex review R1).
   */
  private readonly inTurn = oneAtATimeById();

  constructor(
    private readonly service: Pick<MemberService, "list" | "update" | "remove">,
    /** The slug of the workspace whose members these are. */
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
   * out is newer than what it read: the list kept stays the one with it.
   */
  async load(): Promise<WorkspaceMember[]> {
    const answeredBefore = this.changesAnswered;
    const list = await this.service.list(this.slug);
    if (this.changesAnswered !== answeredBefore && this.list !== undefined) {
      return this.list;
    }
    runInAction(() => {
      this.list = list;
    });
    return list;
  }

  async changeRole(id: string, role: WorkspaceRole): Promise<void> {
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
        if (!(error instanceof ApiError && error.code === "workspace.member_not_found")) {
          throw error;
        }
      }
      this.changed((list) => list.filter((member) => member.id !== id));
    });
  }

  private changed(change: (list: WorkspaceMember[]) => WorkspaceMember[]): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.list !== undefined) {
        this.list = change(this.list);
      }
    });
  }
}
