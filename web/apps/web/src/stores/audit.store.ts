import { makeAutoObservable, runInAction } from "mobx";

import type { NotebookAuditEvent, NotebookAuditEventPage, OwnerlessService } from "../services/ownerless.service";

/**
 * AuditStore holds the audit events of a workspace's ownerless notebooks
 * that its admins have read, the newest first, a page at a time (M3/P5
 * design 3.2): the first page, which a read again puts in place, and the
 * pages after it that more adds.
 */
export class AuditStore {
  events: NotebookAuditEvent[] | undefined = undefined;
  /** The cursor of the page after the events held; null on the last page, or before the first. */
  nextCursor: string | null = null;
  /** How many first pages have been asked for: one answered for an earlier ask is dropped. */
  private reads = 0;
  /** The page on its way, if one is, with its cursor. */
  private next: { cursor: string; added: Promise<void> } | undefined = undefined;

  constructor(
    private readonly service: Pick<OwnerlessService, "auditEvents">,
    /** The slug of the workspace whose audit events these are. */
    private readonly slug: string
  ) {
    makeAutoObservable<this, "service" | "slug" | "reads" | "next">(this, {
      service: false,
      slug: false,
      reads: false,
      next: false,
    });
  }

  /** load reads the first page and puts it in place (see receiveFirst); SWR calls it. A later first read wins. */
  async load(): Promise<NotebookAuditEvent[]> {
    const read = ++this.reads;
    const page = await this.service.auditEvents(this.slug);
    if (read === this.reads) {
      this.receiveFirst(page);
    }
    return this.events ?? page.data;
  }

  /**
   * receiveFirst puts the first page in place of the events held. When the
   * events held go on past its last, as they do once more has added pages
   * and fewer events than a page came since, the ones after it stay, their
   * cursor with them: a read again, as on the window's focus, keeps what
   * was loaded (M3/P5 review Q2). Otherwise the first page replaces them.
   */
  private receiveFirst(page: NotebookAuditEventPage): void {
    const last = page.data.at(-1);
    const held = this.events ?? [];
    const at = page.next_cursor === null || last === undefined ? -1 : held.findIndex((e) => e.id === last.id);
    if (at === -1) {
      this.events = page.data;
      this.nextCursor = page.next_cursor;
    } else {
      this.events = [...page.data, ...held.slice(at + 1)];
    }
  }

  /**
   * more reads the page after the events held and adds its events, each
   * once; the page on its way for the same cursor is the one asked for.
   * Nothing on the last page.
   */
  more(): Promise<void> {
    const cursor = this.nextCursor;
    if (cursor === null) {
      return Promise.resolve();
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
   * addPage adds the events of the page cursor names, unless the events
   * held no longer end where it begins: a first page read again replaced
   * them meanwhile (M3/P5 review M2).
   */
  private async addPage(cursor: string): Promise<void> {
    const page = await this.service.auditEvents(this.slug, cursor);
    if (cursor !== this.nextCursor) {
      return;
    }
    runInAction(() => {
      const held = this.events ?? [];
      const known = new Set(held.map((event) => event.id));
      this.events = [...held, ...page.data.filter((event) => !known.has(event.id))];
      this.nextCursor = page.next_cursor;
    });
  }
}
