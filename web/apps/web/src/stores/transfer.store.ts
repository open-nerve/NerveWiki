import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATime, oneAtATimeById } from "../lib/one-at-a-time";
import type { TransferJob, TransferJobDetail, TransferJobPage, TransferService } from "../services/transfer.service";

/** under tells whether a job is under way: queued or running. */
function under(job: TransferJob): boolean {
  return job.state === "queued" || job.state === "running";
}

/**
 * TransferStore holds the jobs of a notebook that the account sees, the
 * newest first, a page at a time (M7/P5 design 4.2). A read again reads
 * back as many pages as are held, as the backlinks are read (v0.1 design
 * 13.2, item 19): a job under way, or an address that expires, on a page
 * that more added is read again too. An export started, or a job
 * cancelled, here is put in place at once; it may come before the list.
 */
export class TransferStore {
  jobs: TransferJob[] | undefined = undefined;
  /** The cursor of the page after the jobs held; null on the last page, or before the first. */
  nextCursor: string | null = null;
  /** Whether a list was read: before, the jobs held are those started here, if any. */
  loaded = false;
  /** How many pages the jobs held came in: the first, and each that more added. */
  private pages = 0;
  /** How many reads of the list have been asked for: one answered for an earlier ask is dropped, the later wins. */
  private reads = 0;
  /** How many writes have put a job in place: a read out before one, which may not have it, is dropped. */
  private writes = 0;
  /** The page on its way, if one is, with its cursor. */
  private next: { cursor: string; added: Promise<TransferJob[]> } | undefined = undefined;
  /** The exports' starts go one at a time; each job's cancels, too (v0.1 design 13.2, item 1). */
  private readonly starts = oneAtATime();
  private readonly cancels = oneAtATimeById();

  constructor(
    private readonly service: Pick<TransferService, "list" | "startExport" | "cancel" | "get">,
    /** The notebook whose jobs these are. */
    private readonly notebookId: string
  ) {
    makeAutoObservable<this, "service" | "notebookId" | "pages" | "reads" | "writes" | "next" | "starts" | "cancels">(
      this,
      {
        service: false,
        notebookId: false,
        pages: false,
        reads: false,
        writes: false,
        next: false,
        starts: false,
        cancels: false,
      }
    );
  }

  /** active tells whether a job held is queued or running: its list is read again every second meanwhile. */
  get active(): boolean {
    return this.jobs?.some(under) ?? false;
  }

  /**
   * load reads the list from its first page, as many pages as are held
   * (at least one; one more added meanwhile is read on to), and puts it in
   * place of the jobs held; SWR calls it. A later read wins: one it
   * replaced that fails fails quietly, the later tells. A write answered
   * meanwhile drops it, and before the list is held, reads it again.
   */
  async load(): Promise<TransferJob[]> {
    const read = ++this.reads;
    const wrote = this.writes;
    const jobs: TransferJob[] = [];
    const ids = new Set<string>();
    let cursor: string | undefined;
    let next: string | null;
    let pages = 0;
    do {
      let page: TransferJobPage;
      try {
        // oxlint-disable-next-line no-await-in-loop -- each page's cursor is the one before's
        page = await this.service.list(this.notebookId, cursor);
      } catch (error) {
        if (read !== this.reads) {
          return this.jobs ?? [];
        }
        throw error;
      }
      if (read !== this.reads) {
        return this.jobs ?? [];
      }
      if (wrote !== this.writes) {
        return this.loaded ? (this.jobs ?? []) : this.load();
      }
      pages += 1;
      for (const job of page.data) {
        if (!ids.has(job.id)) {
          ids.add(job.id);
          jobs.push(job);
        }
      }
      next = page.next_cursor;
      cursor = next ?? undefined;
    } while (next !== null && pages < this.pages);
    runInAction(() => {
      this.jobs = jobs;
      this.nextCursor = next;
      this.pages = pages;
      this.loaded = true;
    });
    return jobs;
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
   * longer end where it begins: a read again replaced them meanwhile.
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
      this.pages += 1;
      return added;
    });
  }

  /**
   * start starts the export of the notebook, or of the page rootId and its
   * subtree, and answers its job, which goes first in the jobs held, once
   * (a read may have it already); a read on its way, which may not have
   * it, is dropped.
   */
  start(rootId: string | null): Promise<TransferJob> {
    return this.starts(async () => {
      const job = await this.service.startExport(this.notebookId, rootId);
      runInAction(() => {
        this.writes += 1;
        this.jobs = [job, ...(this.jobs ?? []).filter((held) => held.id !== job.id)];
      });
      return job;
    });
  }

  /** detail reads the job id with its report's problems, which the store does not hold: a report shown reads it. */
  detail(id: string): Promise<TransferJobDetail> {
    return this.service.get(id);
  }

  /**
   * cancel cancels the job id, which the job answered replaces, unless a
   * read found it ended meanwhile: the answer, older, does not bring it
   * back under way. A read on its way is dropped.
   */
  cancel(id: string): Promise<TransferJob> {
    return this.cancels(id, async () => {
      const job = await this.service.cancel(id);
      runInAction(() => {
        this.writes += 1;
        this.jobs = this.jobs?.map((held) => (held.id === job.id && (under(held) || !under(job)) ? job : held));
      });
      return job;
    });
  }
}
