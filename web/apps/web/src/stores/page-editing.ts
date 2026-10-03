import { makeAutoObservable, observableRef, runInAction } from "mobx";

import { ApiError } from "../services/api";
import type { PageContent, PageService } from "../services/page.service";

/** EditingService is the part of PageService an edit uses. */
export type EditingService = Pick<
  PageService,
  "getPageContent" | "putPageContent" | "openEditSession" | "heartbeatEditSession" | "endEditSession"
>;

/**
 * How often the editor beats to keep its edit session (M4 design 4): the
 * server's lease is 60 seconds, so that two beats may be lost before it
 * ends. The server holds the same numbers (EditSessionHeartbeat and
 * EditSessionLease in server/internal/modules/page/domain/session.go);
 * each side's tests pin its own.
 */
export const editSessionHeartbeat = 20_000;

/** How many times a save waits out a 503 server_busy before it is shown as failed. */
const busyRetries = 3;

/** A text to save, and the editor's version it is. */
type Draft = { text: string; version: number };

function refused(error: unknown, status: number, code?: string): boolean {
  return error instanceof ApiError && error.status === status && (code === undefined || error.code === code);
}

/**
 * PageEditing is one edit of a page (M4/P6 design 3.6), from entering
 * the editor to leaving it: the content read as it began, the revision a
 * save goes on, the edit session, the saves. It holds no editor: text
 * comes in as written, with the editor's version, which counts its
 * changes.
 *
 * The session opens as the edit begins and beats every
 * editSessionHeartbeat, and again whenever the tab is shown, since a
 * hidden tab's timers are slowed. One not found is opened anew; a save
 * whose session ended opens one and is sent once more. One save is out
 * at a time; a save asked for meanwhile goes once it is answered, with
 * the latest text asked for.
 */
export class PageEditing {
  /** The content as the edit began; undefined until it is read. */
  content: PageContent | undefined = undefined;
  /** Why the content could not be read. */
  readFailure: unknown = undefined;
  /** The editor's version, and the version last saved: the edit is unsaved while they differ. */
  version = 0;
  savedVersion = 0;
  /** Whether a save is out, and whether it waits out a 503. */
  saving = false;
  busy = false;
  /** Whether a save went through since the edit began. */
  saved = false;
  /** Why the last save failed; cleared by the next. */
  failure: unknown = undefined;
  /** Whether the account may no longer edit the page: a heartbeat was forbidden. */
  lostAccess = false;

  private base = 0;
  private session: string | undefined = undefined;
  private opening: Promise<string> | undefined = undefined;
  private ended = false;
  private beating: ReturnType<typeof setInterval> | undefined = undefined;
  private out: Promise<boolean> | undefined = undefined;
  private next: Draft | undefined = undefined;
  private nextSent: Promise<boolean> | undefined = undefined;

  constructor(
    private readonly service: EditingService,
    readonly pageId: string
  ) {
    makeAutoObservable<
      this,
      "service" | "base" | "session" | "opening" | "ended" | "beating" | "out" | "next" | "nextSent"
    >(
      this,
      {
        service: false,
        pageId: false,
        content: observableRef,
        readFailure: observableRef,
        failure: observableRef,
        base: false,
        session: false,
        opening: false,
        ended: false,
        beating: false,
        out: false,
        next: false,
        nextSent: false,
      },
      { autoBind: true }
    );
  }

  get unsaved(): boolean {
    return this.version !== this.savedVersion;
  }

  /** start begins the edit: it opens the session, starts its heartbeat and reads the content. */
  start(): void {
    this.beating = setInterval(() => void this.beat(), editSessionHeartbeat);
    document.addEventListener("visibilitychange", this.shown);
    void this.sessionId().catch(() => undefined);
    void this.read();
  }

  /** read reads the content the edit begins on, again after it failed. */
  async read(): Promise<void> {
    this.readFailure = undefined;
    try {
      const content = await this.service.getPageContent(this.pageId);
      runInAction(() => {
        this.content = content;
        this.base = content.revision;
      });
    } catch (error) {
      runInAction(() => (this.readFailure = error));
    }
  }

  /** changed is told the editor's version after each change. */
  changed(version: number): void {
    this.version = version;
  }

  /**
   * save saves text, the editor's version version, and resolves to
   * whether it went through (nothing to save goes through at once).
   */
  save(text: string, version: number): Promise<boolean> {
    if (this.out !== undefined) {
      this.next = { text, version };
      this.nextSent ??= this.out.then(() => {
        const next = this.next as Draft;
        this.next = undefined;
        this.nextSent = undefined;
        return this.save(next.text, next.version);
      });
      return this.nextSent;
    }
    if (version === this.savedVersion) {
      return Promise.resolve(true);
    }
    const out = this.send({ text, version }, false, 0).finally(() => (this.out = undefined));
    this.out = out;
    return out;
  }

  /** end ends the edit: the heartbeat stops, the session ends, unanswered. */
  end(): void {
    this.ended = true;
    clearInterval(this.beating);
    document.removeEventListener("visibilitychange", this.shown);
    // A session still opening ends as it opens (sessionId).
    const id = this.session;
    this.session = undefined;
    if (id !== undefined) {
      void this.service.endEditSession(id).catch(() => undefined);
    }
  }

  private async send(draft: Draft, reopened: boolean, waited: number): Promise<boolean> {
    runInAction(() => {
      this.saving = true;
      this.failure = undefined;
    });
    try {
      const page = await this.service.putPageContent(this.pageId, {
        content: draft.text,
        base_revision: this.base,
        edit_session_id: await this.sessionId(),
      });
      runInAction(() => {
        this.base = page.revision;
        this.savedVersion = draft.version;
        this.saved = true;
        this.saving = false;
        this.busy = false;
      });
      return true;
    } catch (error) {
      if (!reopened && refused(error, 409, "page.edit_session_ended")) {
        this.session = undefined;
        return this.send(draft, true, waited);
      }
      if (waited < busyRetries && error instanceof ApiError && error.status === 503 && error.retryAfter !== undefined) {
        runInAction(() => (this.busy = true));
        await new Promise((resolve) => setTimeout(resolve, (error.retryAfter ?? 0) * 1000));
        return this.send(draft, reopened, waited + 1);
      }
      runInAction(() => {
        this.failure = error;
        this.saving = false;
        this.busy = false;
      });
      return false;
    }
  }

  /** sessionId is the edit's session, opened if it has none. */
  private sessionId(): Promise<string> {
    if (this.session !== undefined) {
      return Promise.resolve(this.session);
    }
    this.opening ??= this.service
      .openEditSession(this.pageId)
      .then((session) => {
        if (this.ended) {
          void this.service.endEditSession(session.id).catch(() => undefined);
        } else {
          this.session = session.id;
        }
        return session.id;
      })
      .finally(() => (this.opening = undefined));
    return this.opening;
  }

  private async beat(): Promise<void> {
    const id = this.session;
    if (id === undefined || this.ended) {
      return;
    }
    try {
      await this.service.heartbeatEditSession(id);
    } catch (error) {
      if (refused(error, 404, "page.edit_session_not_found")) {
        if (this.session === id) {
          this.session = undefined;
          void this.sessionId().catch(() => undefined);
        }
      } else if (refused(error, 403)) {
        clearInterval(this.beating);
        runInAction(() => (this.lostAccess = true));
      }
      // Any other failure, a network's, is tried again at the next beat.
    }
  }

  private shown(): void {
    if (document.visibilityState === "visible") {
      void this.beat();
    }
  }
}
