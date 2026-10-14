import { makeAutoObservable, runInAction } from "mobx";

import { oneAtATime, oneAtATimeById } from "../lib/one-at-a-time";
import type {
  TransferJob,
  TransferJobDetail,
  TransferJobPage,
  TransferService,
  UploadOptions,
} from "../services/transfer.service";

/** under tells whether a job is under way: queued or running. */
export function under(job: TransferJob): boolean {
  return job.state === "queued" || job.state === "running";
}

/**
 * How far apart the server's clock and the browser's may be for an ended
 * job to be a lost start's (startedJob): a minute.
 */
const clockMargin = 60_000;

/** Press is a start of a restore or a revert as pressed: the browser's time, and the ids of the jobs held then. */
export type Press = { at: number; known: ReadonlySet<string> };

/** The kinds of the jobs that write a notebook's tree, one of which a notebook has under way at once (M8/P4 design 3.2). */
const writingKinds: ReadonlySet<TransferJob["kind"]> = new Set(["import", "trash_restore", "revert"]);

/**
 * TransferStore holds the jobs of a notebook that the account sees, the
 * newest first, a page at a time (M7/P5 design 4.2). A read again reads
 * back as many pages as are held, as the backlinks are read (v0.1 design
 * 13.2, item 19): a job under way, or an address that expires, on a page
 * that more added is read again too. An export started, or a job
 * cancelled, here is put in place at once; it may come before the list;
 * so are a trash item's restore and a changeset's revert started in the
 * background from the trash and the history (M8/P4 design 4.1). An
 * import's upload has the browser ask before the page is left (warn).
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
  /** How many imports' uploads are going: the browser asks before the page is left meanwhile. */
  uploading = 0;
  /** The page on its way, if one is, with its cursor. */
  private next: { cursor: string; added: Promise<TransferJob[]> } | undefined = undefined;
  /**
   * The starts of the exports, the restores and the reverts go one at a
   * time; each job's cancels, too (v0.1 design 13.2, item 1).
   */
  private readonly starts = oneAtATime();
  private readonly cancels = oneAtATimeById();

  constructor(
    private readonly service: Pick<
      TransferService,
      "list" | "startExport" | "startImport" | "startTrashRestore" | "startRevert" | "cancel" | "get"
    >,
    /** The notebook whose jobs these are. */
    private readonly notebookId: string,
    /** warn asks the browser to ask before the page is left, or no longer (v0.1 design 13.2, item 21). */
    private readonly warn: (on: boolean) => void = () => undefined
  ) {
    makeAutoObservable<
      this,
      "service" | "notebookId" | "warn" | "pages" | "reads" | "writes" | "next" | "starts" | "cancels"
    >(this, {
      service: false,
      notebookId: false,
      warn: false,
      pages: false,
      reads: false,
      writes: false,
      next: false,
      starts: false,
      cancels: false,
    });
  }

  /** active tells whether a job held is queued or running: its list is read again every second meanwhile. */
  get active(): boolean {
    return this.jobs?.some(under) ?? false;
  }

  /**
   * writing tells whether a job held that writes the notebook's tree, an
   * import, a restore or a revert, is queued or running: the server
   * refuses another (transfer.busy; M8/P4 design 4.3). The account sees
   * its own; the notebook's admins see everyone's.
   */
  get writing(): boolean {
    return this.jobs?.some((job) => writingKinds.has(job.kind) && under(job)) ?? false;
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
    return this.starts(async () => this.first(await this.service.startExport(this.notebookId, rootId)));
  }

  /**
   * startTrashRestore starts the restore of the trash item of the root
   * nodeId in the background, and answers its job, which goes first in the
   * jobs held, as an export's (M8/P4 design 4.2).
   */
  startTrashRestore(nodeId: string): Promise<TransferJob> {
    return this.starts(async () => this.first(await this.service.startTrashRestore(this.notebookId, nodeId)));
  }

  /**
   * startRevert starts the revert of the changeset changesetId in the
   * background, with message, and answers its job, which goes first in the
   * jobs held, as an export's (M8/P4 design 4.2).
   */
  startRevert(changesetId: string, message?: string): Promise<TransferJob> {
    return this.starts(async () => this.first(await this.service.startRevert(this.notebookId, changesetId, message)));
  }

  /**
   * press is a start of a restore or a revert pressed now, as startedJob
   * finds its job if its answer is lost: the browser's time, and the jobs
   * held then.
   */
  press(): Press {
    return { at: Date.now(), known: new Set((this.jobs ?? []).map((job) => job.id)) };
  }

  /**
   * startedJob is what press, a start of a restore or a revert whose answer
   * was lost, came to (M8/P4 design 4.2, review B-M6): the list read again,
   * the job of kind on target (a restore's trash item's root, a revert's
   * changeset) that the account me started, and that is press's; undefined
   * when there is none. A read that fails rejects: nothing is known.
   *
   * The server's clock and the browser's differ, so the server's time of a
   * job is not compared with the press's (P4B review A-M1). A job under way
   * is press's whatever its time: a notebook has one job that writes its
   * tree at once (transfer.busy), so a press that started none was refused
   * for it, and it is under way all the same. One ended is press's only if
   * the store did not hold it at the press and the server made it no
   * earlier than clockMargin before the press.
   */
  async startedJob(
    kind: "trash_restore" | "revert",
    target: string,
    me: string,
    press: Press
  ): Promise<TransferJob | undefined> {
    const jobs = await this.load();
    return jobs.find(
      (job) =>
        job.kind === kind &&
        (kind === "revert" ? job.target_changeset_id : job.root_id) === target &&
        job.created_by.user_id === me &&
        (under(job) || (!press.known.has(job.id) && Date.parse(job.created_at) >= press.at - clockMargin))
    );
  }

  /**
   * first puts job, just answered, first in the jobs held, once (a read may
   * have it already), and answers it; a read on its way, which may not have
   * it, is dropped.
   */
  private first(job: TransferJob): TransferJob {
    runInAction(() => {
      this.writes += 1;
      this.jobs = [job, ...(this.jobs ?? []).filter((held) => held.id !== job.id)];
    });
    return job;
  }

  /**
   * startImport uploads file to import under the page parent (null: the
   * notebook's root) and answers its job, which goes first in the jobs
   * held, once; a read on its way is dropped. It waits for no export's
   * start, and they not for it: an upload takes minutes (v0.1 design 13.2,
   * item 1, as an attachment's upload). While it goes, the browser asks
   * before the page is left; options stop it and tell its progress.
   */
  async startImport(parent: string | null, file: File, options?: UploadOptions): Promise<TransferJob> {
    this.uploading += 1;
    this.warn(true);
    try {
      return this.first(await this.service.startImport(this.notebookId, parent, file, options));
    } finally {
      runInAction(() => {
        this.uploading -= 1;
        this.warn(this.uploading > 0);
      });
    }
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
