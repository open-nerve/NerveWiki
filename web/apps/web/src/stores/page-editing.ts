import { makeAutoObservable, observableRef, runInAction } from "mobx";

import { ApiError } from "../services/api";
import type { PageContent, PageService } from "../services/page.service";
import type { EditSession, Opening } from "./edit-session";

/** EditingService is the part of PageService an edit's content uses. */
export type EditingService = Pick<PageService, "getPageContent" | "putPageContent">;

/** EditRecord is where a generation of the stores keeps the edits that hold their page's lock. */
export type EditRecord = { add(editing: PageEditing): unknown; delete(editing: PageEditing): unknown };

/** How many times a save waits out a 503 server_busy before it is shown as failed. */
const busyRetries = 3;

/** A text to save, and the editor's version it is. */
type Draft = { text: string; version: number };

/**
 * A Conflict is a save refused because the page changed since the edit
 * read it (M4/P6 design 3.8): the page's content as it is now, read then,
 * with its revision, and the user's text that was refused.
 */
export type Conflict = { theirs: string; revision: number; mine: string };

function refused(error: unknown, status: number, code?: string): boolean {
  return error instanceof ApiError && error.status === status && (code === undefined || error.code === code);
}

/**
 * PageEditing is one edit of a page (M4/P6 design 3.6; M5/P4 design 3.4),
 * from entering the editor to leaving it: its session, which holds the
 * page's lock; the content read once the lock is the edit's; the revision
 * a save goes on; the saves. It holds no editor: text comes in as
 * written, with the editor's version, which counts its changes.
 *
 * A save whose session ended opens one and is sent once more; a save
 * that loses the session (EditSession) shows no failure: the edit is
 * lost, and saves nothing more. One save is out at a time; a save asked
 * for meanwhile goes once it is answered, with the latest text asked for.
 * A save refused because the page changed reads the page as it is now:
 * the edit is then in conflict, and saves nothing until the user keeps
 * their text or discards it.
 *
 * Once the edit ends, nothing more is sent: a save waiting for its turn,
 * a session or a busy server is not. The view that shows the edit keeps
 * it while mounted: one unmounted and mounted again in the same task, as
 * StrictMode does, keeps its session, which another open would find
 * locked by itself.
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
  conflict: Conflict | undefined = undefined;
  readonly pageId: string;

  private base = 0;
  private ended = false;
  private beginning: Promise<Opening> | undefined = undefined;
  private letting: ReturnType<typeof setTimeout> | undefined = undefined;
  private reading: Promise<void> | undefined = undefined;
  private woken: (() => void) | undefined = undefined;
  private out: Promise<boolean> | undefined = undefined;
  private next: Draft | undefined = undefined;
  private nextSent: Promise<boolean> | undefined = undefined;
  /** The shown editor's quiet save of what it holds: the sign-out saves through it. */
  private saveShown: (() => Promise<boolean>) | undefined = undefined;

  constructor(
    private readonly service: EditingService,
    readonly session: EditSession,
    readonly notebookId: string,
    private readonly record?: EditRecord
  ) {
    this.pageId = session.pageId;
    makeAutoObservable<
      this,
      | "service"
      | "record"
      | "base"
      | "ended"
      | "beginning"
      | "letting"
      | "reading"
      | "woken"
      | "out"
      | "next"
      | "nextSent"
      | "saveShown"
    >(
      this,
      {
        service: false,
        session: false,
        notebookId: false,
        record: false,
        pageId: false,
        content: observableRef,
        readFailure: observableRef,
        failure: observableRef,
        conflict: observableRef,
        base: false,
        ended: false,
        beginning: false,
        letting: false,
        reading: false,
        woken: false,
        out: false,
        next: false,
        nextSent: false,
        saveShown: false,
      },
      { autoBind: true }
    );
  }

  get unsaved(): boolean {
    return this.version !== this.savedVersion;
  }

  /**
   * begin begins the edit, once: it opens the session, with takeOver or
   * not, and reads the content once the lock is the edit's. It answers
   * whether it is, or who holds it. The edit is recorded as it begins, so
   * that a sign-out meanwhile ends it too, and until it ends unless its
   * session did not open.
   */
  begin(takeOver: boolean): Promise<Opening> {
    this.beginning ??= this.opens(takeOver);
    return this.beginning;
  }

  private async opens(takeOver: boolean): Promise<Opening> {
    this.record?.add(this);
    let opening: Opening | undefined;
    try {
      opening = await this.session.open(takeOver);
      return opening;
    } finally {
      runInAction(() => {
        if (opening?.opened !== true || this.ended) {
          this.record?.delete(this);
        } else {
          void this.read();
        }
      });
    }
  }

  /** keep keeps the edit for the view that shows it: a letGo just before is undone. */
  keep(): void {
    clearTimeout(this.letting);
    this.letting = undefined;
  }

  /** letGo ends the edit unless it is kept again within the task: its view is gone for good. */
  letGo(): void {
    clearTimeout(this.letting);
    this.letting = setTimeout(() => void this.end(), 0);
  }

  /**
   * read reads the content the edit begins on, again after it failed; a
   * read out is the one asked for, so that the editor's content and the
   * revision a save goes on come from one answer.
   */
  read(): Promise<void> {
    this.reading ??= this.readContent().finally(() => (this.reading = undefined));
    return this.reading;
  }

  private async readContent(): Promise<void> {
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
    if (this.conflict !== undefined) {
      return Promise.resolve(false);
    }
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

  /**
   * keepMine saves text over the page as it was when the conflict read it:
   * on that revision, so that a write since is a conflict again.
   */
  keepMine(text: string, version: number): Promise<boolean> {
    if (this.conflict !== undefined) {
      this.base = this.conflict.revision;
      this.conflict = undefined;
    }
    return this.save(text, version);
  }

  /**
   * discardMine ends the conflict for the page as it is now: it answers
   * the content the editor is to load, which the edit goes on from, with
   * nothing unsaved.
   */
  discardMine(): string | undefined {
    const conflict = this.conflict;
    if (conflict === undefined) {
      return undefined;
    }
    this.base = conflict.revision;
    this.savedVersion = this.version;
    this.conflict = undefined;
    this.failure = undefined;
    return conflict.theirs;
  }

  /** savesThrough is told the shown editor's quiet save, and undefined once the editor goes. */
  savesThrough(save: (() => Promise<boolean>) | undefined): void {
    this.saveShown = save;
  }

  /**
   * close saves what is unsaved through the shown editor, then ends the
   * edit: the sign-out's end, which does not drop what was typed since the
   * last save. A save that fails, or is not answered within ms, ends it
   * all the same: the end goes out while the sign-out's token is valid.
   */
  async close(within: number): Promise<void> {
    if (!this.ended && this.unsaved && this.saveShown !== undefined) {
      let timer: ReturnType<typeof setTimeout> | undefined;
      const late = new Promise<void>((resolve) => (timer = setTimeout(resolve, within)));
      await Promise.race([this.saveShown().catch(() => false), late]);
      clearTimeout(timer);
    }
    return this.end();
  }

  /**
   * end ends the edit, once: its session ends, and a save not sent yet is
   * not sent: leaving without saving leaves the page as it was last saved.
   * It resolves once the session's end is answered.
   */
  end(): Promise<void> {
    this.ended = true;
    clearTimeout(this.letting);
    this.woken?.();
    this.record?.delete(this);
    return this.session.end();
  }

  private async send(draft: Draft, reopened: boolean, waited: number): Promise<boolean> {
    runInAction(() => {
      this.saving = true;
      this.failure = undefined;
    });
    let session: string | undefined;
    try {
      session = await this.session.current();
      const page = await this.service.putPageContent(this.pageId, {
        content: draft.text,
        base_revision: this.base,
        edit_session_id: session,
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
      if (this.ended) {
        return this.failed(error);
      }
      if (refused(error, 409, "page.revision_mismatch")) {
        return this.inConflict(draft, error);
      }
      if (!reopened && session !== undefined && refused(error, 409, "page.edit_session_ended")) {
        // A session the heartbeat opened meanwhile is the edit's: it is kept.
        this.session.forget(session);
        return this.send(draft, true, waited);
      }
      if (this.session.settle(error)) {
        // The edit is lost: it says why, not the save.
        return this.stopped();
      }
      if (waited < busyRetries && error instanceof ApiError && error.status === 503 && error.retryAfter !== undefined) {
        runInAction(() => (this.busy = true));
        if (await this.waitOut(error.retryAfter)) {
          return this.failed(error);
        }
        return this.send(draft, reopened, waited + 1);
      }
      return this.failed(error);
    }
  }

  private failed(error: unknown): false {
    runInAction(() => {
      this.failure = error;
      this.saving = false;
      this.busy = false;
    });
    return false;
  }

  private stopped(): false {
    runInAction(() => {
      this.saving = false;
      this.busy = false;
    });
    return false;
  }

  /** waitOut waits seconds, or until the edit ends; it answers whether the edit ended. */
  private waitOut(seconds: number): Promise<boolean> {
    return new Promise((resolve) => {
      const timer = setTimeout(() => {
        this.woken = undefined;
        resolve(false);
      }, seconds * 1000);
      this.woken = () => {
        clearTimeout(timer);
        this.woken = undefined;
        resolve(true);
      };
    });
  }

  /** inConflict reads the page as it is now for the conflict of draft; a read that fails leaves refusal the save's failure. */
  private async inConflict(draft: Draft, refusal: unknown): Promise<boolean> {
    try {
      const now = await this.service.getPageContent(this.pageId);
      runInAction(() => (this.conflict = { theirs: now.content, revision: now.revision, mine: draft.text }));
    } catch {
      runInAction(() => (this.failure = refusal));
    }
    runInAction(() => {
      this.saving = false;
      this.busy = false;
    });
    return false;
  }
}
