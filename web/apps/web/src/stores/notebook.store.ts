import { makeAutoObservable, runInAction } from "mobx";

import type { Locale } from "../i18n/locale";
import { oneAtATimeById } from "../lib/one-at-a-time";
import { ApiError } from "../services/api";
import type { Notebook, NotebookCreate, NotebookService, NotebookUpdate } from "../services/notebook.service";
import type { Upgrade, UpgradeService } from "../services/upgrade.service";
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
  /**
   * Each notebook's changes, one at a time (v0.1 design 13.2, item 1): its
   * general page has two forms, either of which may send while the other's
   * change is out.
   */
  private readonly inTurn = oneAtATimeById();

  constructor(
    private readonly service: Pick<NotebookService, "list" | "create" | "update" | "downgrade" | "remove" | "leave">,
    /** The slug of the workspace whose notebooks these are. */
    private readonly slug: string,
    /** The changes of the notebooks' upgrades, which answer the upgrade (M11/P1 design 4.4). */
    private readonly upgrades: Pick<UpgradeService, "start" | "abandon" | "endRollback" | "submit">
  ) {
    makeAutoObservable<this, "service" | "slug" | "upgrades" | "changesAnswered" | "removed" | "inTurn">(this, {
      service: false,
      slug: false,
      upgrades: false,
      changesAnswered: false,
      removed: false,
      inTurn: false,
    });
  }

  /**
   * wasRemoved tells whether this generation deleted or left the notebook
   * id: the list, once it lacks it, sends its pages home rather than to a
   * 404.
   */
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
   * Before the first list there is none to keep it: the list is read again,
   * after the change.
   */
  async load(): Promise<Notebook[]> {
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

  /** create answers the new notebook, of which the account is the admin. */
  async create(body: NotebookCreate): Promise<Notebook> {
    const created = await this.service.create(this.slug, body);
    this.put(created);
    return created;
  }

  async update(id: string, body: NotebookUpdate): Promise<Notebook> {
    return this.inTurn(id, async () => {
      const updated = await this.service.update(id, body);
      this.put(updated);
      return updated;
    });
  }

  /**
   * downgrade makes the LLM Wiki id a standard notebook, in its turn
   * (M10/P1 design 4.4). A refusal that says the list is behind reads it
   * again: an unseen notebook is dropped, not marked as removed. Only an
   * already completed downgrade, successfully reconciled, is done.
   */
  async downgrade(id: string): Promise<void> {
    await this.inTurn(id, async () => {
      try {
        const answered = await this.service.downgrade(id);
        this.put(answered);
      } catch (error) {
        if (
          !(error instanceof ApiError) ||
          (error.code !== "notebook.not_llm_wiki" && error.code !== "forbidden" && error.code !== "notebook.not_found")
        ) {
          throw error;
        }
        try {
          await this.reread(id);
        } catch {
          throw error;
        }
        if (error.code !== "notebook.not_llm_wiki") {
          throw error;
        }
      }
    });
  }

  /**
   * startUpgrade starts upgrading the standard notebook id to an LLM Wiki,
   * the pages it lacks written in locale, in its turn (M11/P1 design 4.4),
   * and answers the upgrade. The answer is the upgrade, not the notebook:
   * the list is read again, which holds the notebook's upgrade now; a read
   * that fails leaves it to the next (the mode event's). A refusal that
   * says the list is behind (forbidden, notebook.not_found, an LLM Wiki
   * already, an upgrade under way already) reads it again too, as a
   * downgrade's does, and is thrown. The upgrade's own read (SWR's) is
   * its page's to read again.
   */
  async startUpgrade(id: string, locale: Locale): Promise<Upgrade> {
    return this.inTurn(id, () => this.upgraded(id, () => this.upgrades.start(id, locale), behindStart));
  }

  /**
   * abandonUpgrade rolls back the notebook id's upgrade, upgrading or
   * reviewing, or continues its stopped rollback, in its turn (M11/P1
   * design 4.4), and answers the upgrade, rolling back: the list is read
   * again, as after a start. A refusal that says the list is behind
   * (forbidden, notebook.not_found, no upgrade it may abandon) reads it
   * again too, and is thrown; a rollback under way already
   * (transfer.busy) is the upgrade's to read again, not the list's.
   */
  async abandonUpgrade(id: string): Promise<Upgrade> {
    return this.inTurn(id, () => this.upgraded(id, () => this.upgrades.abandon(id), behindRollback));
  }

  /**
   * endUpgradeRollback ends the notebook id's stopped rollback, keeping it
   * as it is, in its turn (M11/P1 design 4.4), and answers the upgrade,
   * rolled back: the list, read again, holds the notebook without it. Its
   * refusals read the list again as abandonUpgrade's.
   */
  async endUpgradeRollback(id: string): Promise<Upgrade> {
    return this.inTurn(id, () => this.upgraded(id, () => this.upgrades.endRollback(id), behindRollback));
  }

  /**
   * submitUpgrade submits the notebook id's upgrade for review, in its turn
   * (M11/P2 design 4.3), and answers the upgrade, reviewing with its report:
   * the list is read again, which holds its new state. A refusal that says
   * the list is behind (forbidden, notebook.not_found, the upgrade in
   * another state) reads it again too, and is thrown; a submission not
   * ready, or of an old index, is upgrading again, which the mode event
   * tells the list; busy, nothing to read.
   */
  async submitUpgrade(id: string): Promise<Upgrade> {
    return this.inTurn(id, () => this.upgraded(id, () => this.upgrades.submit(id), behindSubmit));
  }

  /** upgraded answers the upgrade change answers, the list read again after it, and after a refusal of behind. */
  private async upgraded(id: string, change: () => Promise<Upgrade>, behind: ReadonlySet<string>): Promise<Upgrade> {
    let answered: Upgrade;
    try {
      answered = await change();
    } catch (error) {
      if (error instanceof ApiError && error.code !== undefined && behind.has(error.code)) {
        await this.reread(id).catch(() => undefined);
      }
      throw error;
    }
    await this.reread(id).catch(() => undefined);
    return answered;
  }

  /** reread reads the list again and places the notebook id as it holds it, or drops it, no longer seen. */
  private async reread(id: string): Promise<void> {
    const list = await this.service.list(this.slug);
    const seen = list.find((notebook) => notebook.id === id);
    if (seen === undefined) {
      this.changed((current) => current.filter((notebook) => notebook.id !== id));
    } else {
      this.put(seen);
    }
  }

  /** remove deletes the notebook id; one deleted already, or no longer seen, is gone as well. */
  async remove(id: string): Promise<void> {
    await this.inTurn(id, async () => {
      try {
        await this.service.remove(id);
      } catch (error) {
        if (!isNotFound(error)) {
          throw error;
        }
      }
      this.gone(id);
    });
  }

  /**
   * leave ends the account's membership of the notebook id, then reads the
   * workspace's notebooks again: one the account no longer sees is gone;
   * one open to the workspace it still sees, with its default role, and
   * stays (M3/P4 design 3.2). The list is read, not the notebook, which
   * answers 404 once out of sight: a browser logs that as an error. A
   * notebook already gone is gone as well; a membership that has ended
   * already is the refusal's to tell. The leaving is done once answered: a
   * read that fails after it leaves the notebook as it was until the next
   * read, which sends its pages home if it no longer has it.
   */
  async leave(id: string): Promise<void> {
    await this.inTurn(id, async () => {
      try {
        await this.service.leave(id);
      } catch (error) {
        if (!isNotFound(error)) {
          throw error;
        }
        this.gone(id);
        return;
      }
      let list: Notebook[];
      try {
        list = await this.service.list(this.slug);
      } catch {
        this.removed.add(id);
        return;
      }
      const seen = list.find((notebook) => notebook.id === id);
      if (seen === undefined) {
        this.gone(id);
      } else {
        this.put(seen);
      }
    });
  }

  /** receive places notebook, which the account came to see another way (taking it over), in the list. */
  receive(notebook: Notebook): void {
    this.put(notebook);
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

/** The refusals of an upgrade's start that say the list is behind: the notebook's role, its being, its mode, its upgrade. */
const behindStart: ReadonlySet<string> = new Set([
  "forbidden",
  "notebook.not_found",
  "notebook.already_llm_wiki",
  "notebook.upgrading",
]);

/**
 * The refusals of an upgrade's abandonment and of its rollback's end that
 * say the list is behind: the notebook's role, its being, its upgrade's
 * state.
 */
const behindRollback: ReadonlySet<string> = new Set(["forbidden", "notebook.not_found", "wiki.upgrade_state"]);

/** The refusals of an upgrade's submission that say the list is behind: the notebook's role, its being, its upgrade's state. */
const behindSubmit: ReadonlySet<string> = new Set(["forbidden", "notebook.not_found", "wiki.upgrade_state"]);

/** isNotFound tells whether error is the notebook's 404: deleted, or not seen. */
function isNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === "notebook.not_found";
}
