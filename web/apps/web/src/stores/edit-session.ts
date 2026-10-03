import { action, makeObservable, observableRef } from "mobx";

import type { HubEvent, PageLifecycle } from "../events/hub";
import { ApiError } from "../services/api";
import type { PageService } from "../services/page.service";

/** SessionService is the part of PageService an edit session uses. */
export type SessionService = Pick<PageService, "openEditSession" | "heartbeatEditSession" | "endEditSession">;

/**
 * How often the editor beats to keep its edit session (M4 design 4): the
 * server's lease is 120 seconds (M5 design 4.6), so that a hidden tab's
 * beats, throttled to one a minute, still keep it. The server holds the
 * same numbers (EditSessionHeartbeat and
 * EditSessionLease in server/internal/modules/page/domain/session.go);
 * each side's tests pin its own.
 */
export const editSessionHeartbeat = 20_000;

/** Who holds a page's edit lock, as page.locked names them. */
type LockHolder = { user_id: string; display_name: string };

/**
 * Why an edit may no longer write (M5 design 4.3, 4.9): its session was
 * taken over by the account elsewhere; an admin unlocked it (by, their
 * name); it lapsed and holder took the lock meanwhile; the page is gone;
 * the account may no longer edit it.
 */
export type EditLost =
  | { reason: "taken_over" }
  | { reason: "unlocked"; by: string }
  | { reason: "taken"; holder: LockHolder }
  | { reason: "gone" }
  | { reason: "no_access" };

/** What opening the session answered: the lock, or who holds it. */
export type Opening = { opened: true } | { opened: false; holder: LockHolder };

export type EditSessionDeps = {
  service: SessionService;
  page: PageLifecycle;
  /** Subscribes to the events of the tab's login, where it has a stream; returns the unsubscribe. */
  events?: (listener: (event: HubEvent) => void) => () => void;
  /** Ends the session id as the page is left: at once, unanswered, and only with a token still valid. */
  leave: (id: string) => void;
};

/** EditEnded is what an edit ended answers a save or an open still out. */
export class EditEnded extends Error {
  constructor() {
    super("The edit has ended.");
    this.name = "EditEnded";
  }
}

function refused(error: unknown, status: number, code?: string): error is ApiError {
  return error instanceof ApiError && error.status === status && (code === undefined || error.code === code);
}

/**
 * lostBy is why error ends an edit for good, or undefined for one it goes
 * on from: a session taken over or unlocked, the lock someone else's, the
 * page gone, the account's access to it.
 */
function lostBy(error: unknown): EditLost | undefined {
  if (refused(error, 409, "page.edit_session_taken_over")) {
    return { reason: "taken_over" };
  }
  if (refused(error, 409, "page.edit_session_unlocked")) {
    return { reason: "unlocked", by: error.problem?.ended_by?.display_name ?? "" };
  }
  if (refused(error, 409, "page.locked") && error.problem?.lock !== undefined) {
    return { reason: "taken", holder: error.problem.lock };
  }
  if (refused(error, 404, "page.not_found")) {
    return { reason: "gone" };
  }
  if (refused(error, 403)) {
    return { reason: "no_access" };
  }
  return undefined;
}

/** lapsed is whether error says the session is no longer there, lapsed or ended by its own: one opened anew may take its place. */
function lapsed(error: unknown): boolean {
  return refused(error, 404) || refused(error, 409, "page.edit_session_ended");
}

/**
 * EditSession is the session of one edit of a page (M5 design 4.1–4.3,
 * 4.6, 4.7), which holds the page's edit lock: opened, with take-over or
 * not, before the edit begins; beaten every editSessionHeartbeat, and at
 * once when the tab is shown, when the page's lock changes, when the
 * stream connects; ended with the edit, and as the page is left.
 *
 * A session that lapsed is opened anew, never with take-over: the lock
 * someone took meanwhile is theirs. A session taken over or unlocked,
 * a page gone or out of reach is lost: the edit beats, opens and saves no
 * more, and says why. Back from the back-forward cache, the session beats
 * before anything else: the end sent as the page was left may not have
 * gone out, and the session, still alive, is the edit's again.
 */
export class EditSession {
  /** Why the edit may no longer write; undefined while it may. */
  lost: EditLost | undefined = undefined;

  private id: string | undefined = undefined;
  private opening: Promise<string> | undefined = undefined;
  private ended = false;
  private ending: Promise<void> = Promise.resolve();
  private beating: ReturnType<typeof setInterval> | undefined = undefined;
  private offs: (() => void)[] = [];

  constructor(
    private readonly deps: EditSessionDeps,
    readonly pageId: string
  ) {
    makeObservable<this, "lose">(this, { lost: observableRef, lose: action });
  }

  /**
   * open opens the session as the edit begins: it answers whether the lock
   * is the edit's, or who holds it. With takeOver, the account's sessions
   * of the page elsewhere end first; someone else's lock stays theirs.
   */
  async open(takeOver: boolean): Promise<Opening> {
    try {
      await this.opened(takeOver);
    } catch (error) {
      if (refused(error, 409, "page.locked") && error.problem?.lock !== undefined) {
        return { opened: false, holder: error.problem.lock };
      }
      throw error;
    }
    this.follow();
    return { opened: true };
  }

  /**
   * current answers the session a save goes in: the one the edit has, or
   * one opened anew once it lapsed. It rejects once the edit is lost or
   * ended, and with the open's refusal, which may lose it.
   */
  current(): Promise<string> {
    if (this.ended || this.lost !== undefined) {
      return Promise.reject(new EditEnded());
    }
    if (this.id !== undefined) {
      return Promise.resolve(this.id);
    }
    return this.opened(false).catch((error: unknown) => {
      this.settle(error);
      throw error;
    });
  }

  /** forget lets the session id go, which a save found ended: the next save opens one anew. */
  forget(id: string): void {
    if (this.id === id) {
      this.id = undefined;
    }
  }

  /** settle answers whether error, a save's or an open's, lost the edit: then nothing more is sent. */
  settle(error: unknown): boolean {
    if (this.lost !== undefined) {
      return true;
    }
    const lost = lostBy(error);
    if (lost !== undefined && !this.ended) {
      this.lose(lost);
    }
    return lost !== undefined;
  }

  /**
   * end ends the edit's session: the beats stop, and the session ends,
   * once; it resolves once the end is answered. A session still opening
   * ends as it opens; a lost one has nothing to end.
   */
  end(): Promise<void> {
    if (this.ended) {
      return this.ending;
    }
    this.ended = true;
    this.unfollow();
    const id = this.id;
    this.id = undefined;
    if (id !== undefined) {
      this.ending = this.deps.service.endEditSession(id).catch(() => undefined);
    }
    return this.ending;
  }

  /** opened is the session opened, the one open that is out at a time; an edit ended meanwhile ends it. */
  private opened(takeOver: boolean): Promise<string> {
    this.opening ??= this.deps.service
      .openEditSession(this.pageId, takeOver)
      .then((session) => {
        if (this.ended) {
          void this.deps.service.endEditSession(session.id).catch(() => undefined);
          throw new EditEnded();
        }
        this.id = session.id;
        return session.id;
      })
      .finally(() => (this.opening = undefined));
    return this.opening;
  }

  /** follow starts the beats, and beating at once on what may have changed the session. */
  private follow(): void {
    if (this.ended || this.beating !== undefined) {
      return;
    }
    this.beating = setInterval(() => void this.beat(), editSessionHeartbeat);
    const { page, events } = this.deps;
    this.offs = [
      page.on("visibilitychange", () => {
        if (page.visible()) {
          void this.beat();
        }
      }),
      page.on("pagehide", () => {
        if (this.id !== undefined) {
          this.deps.leave(this.id);
        }
      }),
      page.on("pageshow", ({ persisted }) => {
        if (persisted === true) {
          void this.beat();
        }
      }),
    ];
    if (events !== undefined) {
      this.offs.push(
        events((event) => {
          if (event.type === "connected" || (event.type === "lock" && event.data.page_id === this.pageId)) {
            void this.beat();
          }
        })
      );
    }
  }

  private unfollow(): void {
    clearInterval(this.beating);
    for (const off of this.offs) {
      off();
    }
    this.offs = [];
  }

  private lose(lost: EditLost): void {
    this.lost = lost;
    this.id = undefined;
    this.unfollow();
  }

  /**
   * beat keeps the session. One lapsed is opened anew, never taking the
   * lock over; a session taken over or unlocked, or out of reach, loses
   * the edit; any other failure, a network's, is tried again at the next
   * beat.
   */
  private async beat(): Promise<void> {
    const id = this.id;
    if (id === undefined || this.ended) {
      return;
    }
    try {
      await this.deps.service.heartbeatEditSession(id);
    } catch (error) {
      if (this.id !== id || this.ended) {
        return;
      }
      if (lapsed(error)) {
        this.id = undefined;
        void this.current().catch(() => undefined);
      } else {
        this.settle(error);
      }
    }
  }
}
