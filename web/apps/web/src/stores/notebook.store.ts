import { makeAutoObservable, runInAction } from "mobx";

import { ApiError } from "../services/api";
import type { Notebook, NotebookCreate, NotebookService, NotebookUpdate } from "../services/notebook.service";
import type { Workspace } from "../services/workspace.service";
import { byName } from "./order";

/**
 * NotebookGroups are a workspace's notebooks as the left column shows them
 * (M3 design 5): the account's own, private with one member, and the
 * team's, the rest; each in the list's order.
 */
export type NotebookGroups = { mine: Notebook[]; team: Notebook[] };

export function groupNotebooks(list: readonly Notebook[]): NotebookGroups {
  return { mine: list.filter(isOwn), team: list.filter((notebook) => !isOwn(notebook)) };
}

function isOwn(notebook: Notebook): boolean {
  return notebook.workspace_access === "none" && notebook.member_count === 1;
}

/** createsNotebooks tells whether the account may create notebooks in workspace: as its admin or member, not a guest. */
export function createsNotebooks(workspace: Pick<Workspace, "role">): boolean {
  return workspace.role === "admin" || workspace.role === "member";
}

/**
 * NotebookStore holds the notebooks of one workspace that the account
 * sees, in the server's order, for one generation (M3/P4 design 3.2): the
 * left column lists them, and the notebook pages find theirs among them.
 */
export class NotebookStore {
  list: Notebook[] | undefined = undefined;
  /** How many changes have been answered: a read that overlaps one may have read the list before it. */
  private changesAnswered = 0;
  /** The notebooks this generation deleted or left and no longer sees: their pages go to the workspace's home. */
  private readonly removed = new Set<string>();

  constructor(
    private readonly service: Pick<NotebookService, "list" | "create" | "update" | "remove" | "leave">,
    /** The slug of the workspace whose notebooks these are. */
    private readonly slug: string
  ) {
    makeAutoObservable<this, "service" | "slug" | "changesAnswered" | "removed">(this, {
      service: false,
      slug: false,
      changesAnswered: false,
      removed: false,
    });
  }

  /** wasRemoved tells whether this generation deleted or left the notebook id, and no longer sees it. */
  wasRemoved(id: string): boolean {
    return this.removed.has(id);
  }

  /** byId is the notebook id in the list, if the list has it. */
  byId(id: string): Notebook | undefined {
    return this.list?.find((notebook) => notebook.id === id);
  }

  /**
   * load reads the list; SWR calls it. A change answered while the read was
   * out is newer than what it read: the list kept stays the one with it.
   */
  async load(): Promise<Notebook[]> {
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

  /** create answers the new notebook, of which the account is the admin. */
  async create(body: NotebookCreate): Promise<Notebook> {
    const created = await this.service.create(this.slug, body);
    this.put(created);
    return created;
  }

  async update(id: string, body: NotebookUpdate): Promise<Notebook> {
    const updated = await this.service.update(id, body);
    this.put(updated);
    return updated;
  }

  /** remove deletes the notebook id; one deleted already, or no longer seen, is gone as well. */
  async remove(id: string): Promise<void> {
    try {
      await this.service.remove(id);
    } catch (error) {
      if (!isNotFound(error)) {
        throw error;
      }
    }
    this.gone(id);
  }

  /**
   * leave ends the account's membership of the notebook id, then reads the
   * workspace's notebooks again: one the account no longer sees is gone;
   * one open to the workspace it still sees, with its default role, and
   * stays (M3/P4 design 3.2). The list is read, not the notebook, which
   * answers 404 once out of sight: a browser logs that as an error. A
   * notebook already gone is gone as well; a membership that has ended
   * already is the refusal's to tell.
   */
  async leave(id: string): Promise<void> {
    try {
      await this.service.leave(id);
    } catch (error) {
      if (!isNotFound(error)) {
        throw error;
      }
      this.gone(id);
      return;
    }
    const seen = (await this.service.list(this.slug)).find((notebook) => notebook.id === id);
    if (seen === undefined) {
      this.gone(id);
    } else {
      this.put(seen);
    }
  }

  /** put places notebook in the list by name; a read may hold it already. */
  private put(notebook: Notebook): void {
    this.changed((list) => [...list.filter((each) => each.id !== notebook.id), notebook]);
  }

  private gone(id: string): void {
    this.removed.add(id);
    this.changed((list) => list.filter((notebook) => notebook.id !== id));
  }

  private changed(change: (list: Notebook[]) => Notebook[]): void {
    runInAction(() => {
      this.changesAnswered += 1;
      if (this.list !== undefined) {
        this.list = change(this.list).toSorted(byName);
      }
    });
  }
}

/** isNotFound tells whether error is the notebook's 404: deleted, or not seen. */
function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === "notebook.not_found";
}
