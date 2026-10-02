import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { InvitationLink } from "../services/invitation.service";
import type { SlugAvailability, Workspace, WorkspaceCreate, WorkspaceService } from "../services/workspace.service";
import { byName } from "./order";

/**
 * WorkspaceStore holds the workspaces of one generation's account, in the
 * server's order (M2/P5 design 3.4): the switcher lists them, and the
 * workspace pages find theirs among them.
 */
export class WorkspaceStore {
  list: Workspace[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;
  /** The slugs of the workspaces this generation deleted or left: their pages go to the landing, not to the 404. */
  private readonly removed = new Set<string>();
  /**
   * Each workspace's renames, deletion and leaving, one at a time (v0.1
   * design 13.2, item 1): the store outlives the general page, whose form,
   * mounted anew, may send while the rename before is out (M3 Codex review
   * R1).
   */
  private readonly inTurn = oneAtATimeById();

  constructor(
    private readonly service: Pick<
      WorkspaceService,
      "list" | "create" | "rename" | "remove" | "leave" | "accept" | "checkSlug"
    >
  ) {
    makeAutoObservable<this, "service" | "changesAnswered" | "removed" | "inTurn">(this, {
      service: false,
      changesAnswered: false,
      removed: false,
      inTurn: false,
    });
  }

  /** wasRemoved tells whether this generation deleted or left the workspace of slug. */
  wasRemoved(slug: string): boolean {
    return this.removed.has(slug);
  }

  /** bySlug is the workspace of slug in the list, if the list has it. */
  bySlug(slug: string): Workspace | undefined {
    return this.list?.find((workspace) => workspace.slug === slug);
  }

  /**
   * load reads the list; SWR calls it (useSWR with this as fetcher), on
   * focus too. A change answered while the read was out is newer than what
   * it read: the list kept stays the one with the change.
   */
  async load(): Promise<Workspace[]> {
    const answeredBefore = this.changesAnswered;
    const list = await this.service.list();
    if (this.changesAnswered !== answeredBefore && this.list !== undefined) {
      return this.list;
    }
    runInAction(() => {
      this.list = list;
    });
    return list;
  }

  async create(body: WorkspaceCreate): Promise<Workspace> {
    const created = await this.service.create(body);
    // A read answered before the creation may hold it already.
    this.changed((list) => [...list.filter((workspace) => workspace.id !== created.id), created]);
    return created;
  }

  async rename(slug: string, name: string): Promise<Workspace> {
    return this.inTurn(slug, async () => {
      const renamed = await this.service.rename(slug, name);
      this.changed((list) => list.map((workspace) => (workspace.id === renamed.id ? renamed : workspace)));
      return renamed;
    });
  }

  /** accept joins the workspace link invites to, and answers it with the account's role there. */
  async accept(link: InvitationLink): Promise<Workspace> {
    const joined = await this.service.accept(link);
    // A read answered before the acceptance may hold it already.
    this.changed((list) => [...list.filter((workspace) => workspace.id !== joined.id), joined]);
    return joined;
  }

  /** remove deletes the workspace of slug. */
  remove(slug: string): Promise<void> {
    return this.gone(slug, () => this.service.remove(slug));
  }

  /** leave ends the account's membership of the workspace of slug. */
  leave(slug: string): Promise<void> {
    return this.gone(slug, () => this.service.leave(slug));
  }

  /** checkSlug asks whether slug can name a new workspace; it changes nothing the store holds. */
  checkSlug(slug: string): Promise<SlugAvailability> {
    return this.service.checkSlug(slug);
  }

  /**
   * gone sends request, after which the account no longer has the
   * workspace of slug; one it no longer has already (deleted, or the
   * account removed from it) is gone as well.
   */
  private async gone(slug: string, request: () => Promise<void>): Promise<void> {
    await this.inTurn(slug, async () => {
      try {
        await request();
      } catch (error) {
        if (!(error instanceof ApiError && error.code === "workspace.not_found")) {
          throw error;
        }
      }
      this.removed.add(slug);
      this.changed((list) => list.filter((workspace) => workspace.slug !== slug));
    });
  }

  private changed(change: (list: Workspace[]) => Workspace[]): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.list !== undefined) {
        this.list = change(this.list).toSorted(byName);
      }
    });
  }
}
