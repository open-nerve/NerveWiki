import { makeAutoObservable, runInAction } from "mobx";

import type { NotebookAuditEvent, OwnerlessService } from "../services/ownerless.service";

/**
 * AuditStore holds the audit events of a workspace's ownerless notebooks
 * that its admins have read, the newest first, a page at a time (M3/P5
 * design 3.2): the first page, which a read again replaces, and the pages
 * after it that more adds.
 */
export class AuditStore {
  events: NotebookAuditEvent[] | undefined = undefined;
  /** The cursor of the next page; null on the last page, or before the first. */
  nextCursor: string | null = null;
  /** How many first pages have been asked for: a page answered for an earlier one belongs to an older series. */
  private series = 0;
  /** The page on its way, if one is. */
  private next: Promise<void> | undefined = undefined;

  constructor(
    private readonly service: Pick<OwnerlessService, "auditEvents">,
    /** The slug of the workspace whose audit events these are. */
    private readonly slug: string
  ) {
    makeAutoObservable<this, "service" | "slug" | "series" | "next">(this, {
      service: false,
      slug: false,
      series: false,
      next: false,
    });
  }

  /** load reads the first page, in place of every page held; SWR calls it. A later first read wins. */
  async load(): Promise<NotebookAuditEvent[]> {
    const series = ++this.series;
    const page = await this.service.auditEvents(this.slug);
    if (series !== this.series) {
      return this.events ?? page.data;
    }
    runInAction(() => {
      this.events = page.data;
      this.nextCursor = page.next_cursor;
    });
    return page.data;
  }

  /**
   * more reads the page after those held and adds its events, each once; a
   * page already on its way is the one asked for. Nothing on the last page.
   * A page answered after the first was read again belongs to the older
   * series, and is dropped.
   */
  more(): Promise<void> {
    const cursor = this.nextCursor;
    if (this.next !== undefined || cursor === null) {
      return this.next ?? Promise.resolve();
    }
    this.next = this.addPage(cursor, this.series).finally(() => {
      this.next = undefined;
    });
    return this.next;
  }

  /** addPage adds the events of the page cursor names, unless the first page was read again after series. */
  private async addPage(cursor: string, series: number): Promise<void> {
    const page = await this.service.auditEvents(this.slug, cursor);
    if (series !== this.series) {
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
