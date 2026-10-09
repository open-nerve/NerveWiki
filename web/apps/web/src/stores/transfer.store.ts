import { makeAutoObservable, runInAction } from "mobx";

import type { TransferJob, TransferJobDetail, TransferJobPage, TransferService } from "../services/transfer.service";

/**
 * TransferStore holds the jobs of a notebook that the account sees, the
 * newest first, a page at a time (M7/P5 design 4.2), as AuditStore holds
 * its events: the first page, which a read again puts in place, and the
 * pages after it that more adds. An export started, or a job cancelled,
 * here is put in place at once.
 */
export class TransferStore {
  jobs: TransferJob[] | undefined = undefined;
  /** The cursor of the page after the jobs held; null on the last page, or before the first. */
  nextCursor: string | null = null;
  /**
   * How many first pages have been asked for, and jobs put in place: a
   * first page answered for an earlier ask, which may not have them yet,
   * is dropped.
   */
  private reads = 0;
  /** The page on its way, if one is, with its cursor. */
  private next: { cursor: string; added: Promise<TransferJob[]> } | undefined = undefined;

  constructor(
    private readonly service: Pick<TransferService, "list" | "startExport" | "cancel" | "get">,
    /** The notebook whose jobs these are. */
    private readonly notebookId: string
  ) {
    makeAutoObservable<this, "service" | "notebookId" | "reads" | "next">(this, {
      service: false,
      notebookId: false,
      reads: false,
      next: false,
    });
  }

  /** active tells whether a job held is queued or running: its list is read again every second meanwhile. */
  get active(): boolean {
    return this.jobs?.some((job) => job.state === "queued" || job.state === "running") ?? false;
  }

  /** load reads the first page and puts it in place (see receiveFirst); SWR calls it. A later first read wins. */
  async load(): Promise<TransferJob[]> {
    const read = ++this.reads;
    const page = await this.service.list(this.notebookId);
    if (read === this.reads) {
      this.receiveFirst(page);
    }
    return this.jobs ?? page.data;
  }

  /**
   * receiveFirst puts the first page in place of the jobs held, as
   * AuditStore does its events: the jobs it has replace those held, and
   * the ones held after its last stay, their cursor with them, once more
   * has added pages; otherwise the first page replaces them.
   */
  private receiveFirst(page: TransferJobPage): void {
    const last = page.data.at(-1);
    const held = this.jobs ?? [];
    const at = page.next_cursor === null || last === undefined ? -1 : held.findIndex((job) => job.id === last.id);
    if (at === -1) {
      this.jobs = page.data;
      this.nextCursor = page.next_cursor;
    } else {
      this.jobs = [...page.data, ...held.slice(at + 1)];
    }
  }

  /**
   * more reads the page after the jobs held and adds its jobs, each once;
   * the page on its way for the same cursor is the one asked for. Nothing
   * on the last page. It answers the jobs it added.
   */
  more(): Promise<TransferJob[]> {
    const cursor = this.nextCursor;
    if (cursor === null) {
      return Promise.resolve([]);
    }
    if (this.next?.cursor === cursor) {
      return this.next.added;
    }
    const added = this.addPage(cursor).finally(() => {
      if (this.next?.added === added) {
        this.next = undefined;
      }
    });
    this.next = { cursor, added };
    return added;
  }

  /**
   * addPage adds the jobs of the page cursor names, unless the jobs held no
   * longer end where it begins: a first page read again replaced them
   * meanwhile.
   */
  private async addPage(cursor: string): Promise<TransferJob[]> {
    const page = await this.service.list(this.notebookId, cursor);
    if (cursor !== this.nextCursor) {
      return [];
    }
    return runInAction(() => {
      const held = this.jobs ?? [];
      const known = new Set(held.map((job) => job.id));
      const added = page.data.filter((job) => !known.has(job.id));
      this.jobs = [...held, ...added];
      this.nextCursor = page.next_cursor;
      return added;
    });
  }

  /**
   * start starts the export of the notebook, or of the page rootId and its
   * subtree, and answers its job, which goes first in the jobs held; a
   * first page on its way, which may not have it, is dropped.
   */
  async start(rootId: string | null): Promise<TransferJob> {
    const job = await this.service.startExport(this.notebookId, rootId);
    runInAction(() => {
      this.reads += 1;
      this.jobs = [job, ...(this.jobs ?? []).filter((held) => held.id !== job.id)];
    });
    return job;
  }

  /** detail reads the job id with its report's problems, which the store does not hold: a report shown reads it. */
  detail(id: string): Promise<TransferJobDetail> {
    return this.service.get(id);
  }

  /** cancel cancels the job id, which the job answered replaces; a first page on its way is dropped. */
  async cancel(id: string): Promise<TransferJob> {
    const job = await this.service.cancel(id);
    runInAction(() => {
      this.reads += 1;
      this.jobs = this.jobs?.map((held) => (held.id === job.id ? job : held));
    });
    return job;
  }
}
