import { makeAutoObservable, runInAction } from "mobx";

import { ApiError } from "../services/api";
import type { SlugAvailability, Workspace, WorkspaceCreate, WorkspaceService } from "../services/workspace.service";

/**
 * byName is the order of the server's list: lower(name), name, id. The
 * database collates by code point (C.UTF-8, v0.1 design 7.1), this by
 * UTF-16 code unit, which orders the same but for characters beyond the
 * BMP (an emoji against a full-width letter); its lower() maps one
 * character to one, toLowerCase a few to two. Where that changes the
 * order, the next load puts the list back in the server's.
 */
function byName(a: Workspace, b: Workspace): number {
  const [x, y] = [a.name.toLowerCase(), b.name.toLowerCase()];
  return compare(x, y) || compare(a.name, b.name) || compare(a.id, b.id);
}

function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/**
 * WorkspaceStore holds the workspaces of one generation's account, in the
 * server's order (M2/P5 design 3.4): the switcher lists them, and the
 * workspace pages find theirs among them.
 */
export class WorkspaceStore {
  list: Workspace[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;
  /** The slugs of the workspaces this generation deleted: their pages go to the landing, not to the 404. */
  private readonly removed = new Set<string>();

  constructor(private readonly service: Pick<WorkspaceService, "list" | "create" | "rename" | "remove" | "checkSlug">) {
    makeAutoObservable<this, "service" | "changesAnswered" | "removed">(this, {
      service: false,
      changesAnswered: false,
      removed: false,
    });
  }

  /** wasRemoved tells whether this generation deleted the workspace of slug. */
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
    const renamed = await this.service.rename(slug, name);
    this.changed((list) => list.map((workspace) => (workspace.id === renamed.id ? renamed : workspace)));
    return renamed;
  }

  /**
   * remove deletes the workspace of slug; one the account no longer has
   * (deleted already, or the account removed from it) is gone as well.
   */
  async remove(slug: string): Promise<void> {
    try {
      await this.service.remove(slug);
    } catch (error) {
      if (!(error instanceof ApiError && error.code === "workspace.not_found")) {
        throw error;
      }
    }
    this.removed.add(slug);
    this.changed((list) => list.filter((workspace) => workspace.slug !== slug));
  }

  /** checkSlug asks whether slug can name a new workspace; it changes nothing the store holds. */
  checkSlug(slug: string): Promise<SlugAvailability> {
    return this.service.checkSlug(slug);
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
