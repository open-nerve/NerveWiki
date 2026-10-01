import { makeAutoObservable, runInAction } from "mobx";

import type { SlugAvailability, Workspace, WorkspaceCreate, WorkspaceService } from "../services/workspace.service";

/**
 * byName is the order of the server's list: lower(name), name, id, by code
 * point, as the database collates (C.UTF-8, v0.1 design 7.1). Its lower()
 * maps one character to one, toLowerCase a few to two: where that changes
 * the order, the next load puts the list back in the server's.
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

  constructor(private readonly service: Pick<WorkspaceService, "list" | "create" | "rename" | "remove" | "checkSlug">) {
    makeAutoObservable<this, "service" | "changesAnswered">(this, { service: false, changesAnswered: false });
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
    this.changed((list) => [...list, created]);
    return created;
  }

  async rename(slug: string, name: string): Promise<Workspace> {
    const renamed = await this.service.rename(slug, name);
    this.changed((list) => list.map((workspace) => (workspace.id === renamed.id ? renamed : workspace)));
    return renamed;
  }

  async remove(slug: string): Promise<void> {
    await this.service.remove(slug);
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
