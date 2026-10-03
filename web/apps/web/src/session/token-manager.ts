import type { ApiClient, AuthTokens } from "@nervewiki/api-client";
import type { RefreshLock } from "./refresh-lock";

// The browser's tokens (M1/P5 design 3.2): the access token lives in memory only; localStorage keeps one
// record, nwiki.auth = {refresh_token, login_id}, written in one piece and only under the refresh lock.
// login_id is new at every sign-in and kept by refreshes, so a tab can tell another tab's refresh (same
// login_id) from another tab's sign-in (a new one), which may be another account. An operation that changes
// the session (a refresh, a sign-out, the end of a session) acts only on the session it was called for, and
// every decision reads the record there now, never a copy: a record of another session is followed. After
// each request's await, and after the lock hands a failed first refresh back, the record is read again before
// anything is written or decided: without navigator.locks, the lease is not atomic, and another tab may sign
// in meanwhile (M1/P5 design 3.2). A sign-in writes its new session's record without reading it: the last sign-in
// wins.

/** The localStorage key of the session's record. */
export const AUTH_KEY = "nwiki.auth";
/** An access token closer than this to its end is refreshed before it is used. */
const REFRESH_MARGIN_MS = 30_000;
/**
 * How long a refresh or a logout may take: longer than the server's own 4 s + 2 s (M1/P2 design 3.5). The
 * server's configuration check holds its deadlines under this (webRefreshTimeout in
 * server/internal/platform/config/validate.go): change both together.
 */
const REQUEST_TIMEOUT_MS = 8_000;
/** The longest wait between two refreshes that failed for a passing reason. */
const MAX_BACKOFF_MS = 30_000;

type AuthRecord = { refresh_token: string; login_id: string };

/**
 * starting: the first refresh is under way; signed-in: loginId's record is in use; signed-out: no record;
 * unavailable: the first refresh failed for a passing reason (429, 5xx, no network), the record is kept and
 * the refresh is tried again at retryAt.
 */
type SessionStatus = "starting" | "signed-in" | "signed-out" | "unavailable";
export type SessionState = Readonly<{ status: SessionStatus; loginId?: string; retryAt?: number }>;

/** The session cannot be used for now: a refresh failed for a passing reason; try again from retryAt. */
export class SessionUnavailableError extends Error {
  constructor(readonly retryAt: number) {
    super("The session is unavailable for now.");
    this.name = "SessionUnavailableError";
  }
}

/**
 * The tab is no longer in the session a request was made in (another tab signed in or out, or the session
 * ended meanwhile): the request stops, and the tab is in the session the record holds.
 */
export class SessionChangedError extends Error {
  constructor() {
    super("The session changed.");
    this.name = "SessionChangedError";
  }
}

/**
 * The browser would not write the session's storage (full, or blocked): it cannot keep a session, which lives in
 * the record the tabs share. The tab then ends the session it had, and a sign-in it cannot save is undone.
 */
export class SessionStorageError extends Error {
  constructor(cause: unknown) {
    super("This browser could not save the session.", { cause });
    this.name = "SessionStorageError";
  }
}

export type TokenManagerDeps = {
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">;
  lock: RefreshLock;
  /** A client without the auth middleware: a refresh must not wait on itself. */
  client: ApiClient;
  now: () => number;
  /** n random bytes as hexadecimal (crypto.getRandomValues: non-secure contexts have it too). */
  randomHex: (bytes: number) => string;
};

type Outcome =
  | { kind: "tokens"; tokens: AuthTokens; receivedAt: number }
  | { kind: "unauthorized" }
  | { kind: "unavailable"; retryAfterMs?: number };

export class TokenManager {
  #state: SessionState = { status: "starting" };
  #listeners = new Set<() => void>();
  #access: { token: string; expiresAt: number } | undefined;
  #refreshing: { loginId: string | undefined; promise: Promise<string | undefined> } | undefined;
  #failures = 0;
  #retryAt = 0;
  #retryTimer: ReturnType<typeof setTimeout> | undefined;
  #started: Promise<void> | undefined;

  constructor(private readonly deps: TokenManagerDeps) {}

  /** The session as the tab shows it; a new object on every change. */
  get state(): SessionState {
    return this.#state;
  }

  /** Calls listener after every change of state; returns the unsubscribe. */
  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  /**
   * Decides the session when the app starts, once. Without a record the tab is signed out and asks
   * nothing, so the sign-in page makes no request that fails (S2); with one, the first refresh decides.
   */
  start(): Promise<void> {
    this.#started ??= this.#start();
    return this.#started;
  }

  /** Tries the first refresh again now, e.g. from the "try again" button. */
  retry = async (): Promise<void> => {
    clearTimeout(this.#retryTimer);
    this.#retryAt = 0;
    await this.#firstRefresh();
  };

  /**
   * The access token for a request: the one in memory while more than 30 s of it are left, else a
   * refreshed one; undefined when signed out. The time left is counted on this computer's clock from when
   * the token arrived, so a wrong clock changes nothing (M1/P5 design 3.2).
   */
  async accessToken(): Promise<string | undefined> {
    if (this.#state.status === "signed-out") return undefined;
    const access = this.#access;
    if (access !== undefined && access.expiresAt - this.deps.now() > REFRESH_MARGIN_MS) return access.token;
    return this.#refresh();
  }

  /**
   * The access token of the session loginId as it is in memory now, while it has not expired; undefined
   * otherwise. Synchronous, never refreshed: for the one request that cannot wait for a refresh, the end of
   * an edit session as the page is left (M5 design 4.7).
   */
  currentAccessToken(loginId: string): string | undefined {
    const access = this.#access;
    if (this.#state.loginId !== loginId || access === undefined || access.expiresAt <= this.deps.now()) {
      return undefined;
    }
    return access.token;
  }

  /**
   * A new access token after the server refused `sent` with 401: the one another request got meanwhile, else a
   * refreshed one; undefined when the refresh ended the session, or it had ended already.
   */
  async renew(sent: string): Promise<string | undefined> {
    if (this.#state.status === "signed-out") return undefined;
    if (this.#access !== undefined && this.#access.token !== sent) return this.accessToken();
    this.#access = undefined;
    return this.#refresh();
  }

  /**
   * Keeps the tokens of a sign-in or a sign-up, with a new login_id, under the lock (M1/P5 design 3.6). A
   * browser that will not save them (SessionStorageError) has the new session logged out, best effort.
   */
  async signIn(tokens: AuthTokens): Promise<void> {
    const receivedAt = this.deps.now();
    try {
      await this.deps.lock.run(async () => {
        const record = { refresh_token: tokens.refresh_token, login_id: this.deps.randomHex(16) };
        this.deps.storage.setItem(AUTH_KEY, JSON.stringify(record));
        this.#keep(tokens, receivedAt);
        this.#set({ status: "signed-in", loginId: record.login_id });
      });
    } catch (error) {
      if (error instanceof SessionStorageError) await this.#call("/api/v0/auth/logout", tokens.refresh_token);
      throw error;
    }
  }

  /**
   * Signs the tab's session out: under the lock, so the refresh token handed over is the latest, logs out
   * with it (best effort) and removes the record (M1/P5 design 3.2). When the record is another
   * session's by then (another tab signed in first), the tab follows it and logs nobody out: that sign-in
   * replaced this session's refresh token, so the browser has nothing of it left to log out with. The same
   * holds when another tab signs in while the logout is out (the lease is not atomic): the new record stays.
   */
  async signOut(): Promise<void> {
    const loginId = this.#state.loginId;
    await this.deps.lock.run(async () => {
      const record = this.#read();
      if (!isRecordOf(record, loginId)) {
        this.#switchTo(record);
        return;
      }
      await this.#call("/api/v0/auth/logout", record.refresh_token);
      this.#end(loginId);
    });
  }

  /**
   * Ends the session loginId after the server refused a request made in it again with the refreshed token:
   * under the lock, removes the record if it is still that session's, else follows the record. Resolves
   * whether it ended that session: false when the record was no longer that session's.
   */
  async endSession(loginId: string | undefined): Promise<boolean> {
    return this.deps.lock.run(async () => this.#end(loginId));
  }

  /**
   * Another tab changed nwiki.auth (the storage event): removed, it signed out; a new login_id, it signed
   * in, maybe as another account; the same login_id, it only refreshed. The event only says that the record
   * changed: the tab reads the record there now, since an event can arrive after a later write, even after
   * the tab's own sign-in.
   */
  handleStorageChange(): void {
    const record = this.#read();
    if (record === undefined ? this.#state.status !== "signed-out" : record.login_id !== this.#state.loginId) {
      this.#switchTo(record);
    }
  }

  async #start(): Promise<void> {
    const record = this.#read();
    if (record === undefined) {
      this.#set({ status: "signed-out" });
      return;
    }
    this.#set({ status: "starting", loginId: record.login_id });
    await this.#firstRefresh();
  }

  /**
   * The refresh that decides a starting or unavailable session; a passing failure makes it unavailable, while
   * the record is still that session's. navigator.locks hands the failure back in a later task, after another
   * tab's change of the record may have come in: the tab then follows the record.
   */
  async #firstRefresh(): Promise<void> {
    const loginId = this.#state.loginId;
    try {
      await this.#refresh();
    } catch (error) {
      // A change of session, or a browser that cannot keep one: the tab is in the session it follows already.
      if (error instanceof SessionChangedError || error instanceof SessionStorageError) return;
      if (!(error instanceof SessionUnavailableError)) throw error;
      const record = this.#read();
      if (!isRecordOf(record, loginId)) {
        this.#switchTo(record);
        return;
      }
      this.#set({ status: "unavailable", loginId, retryAt: error.retryAt });
      this.#retryTimer = setTimeout(() => void this.retry(), error.retryAt - this.deps.now());
    }
  }

  /**
   * One refresh at a time for the tab's session: a request made after another tab signed in waits for a
   * refresh of the new record, not for the old session's refresh, which ends in SessionChangedError.
   */
  #refresh(): Promise<string | undefined> {
    const loginId = this.#state.loginId;
    const current = this.#refreshing;
    if (current !== undefined && current.loginId === loginId) return current.promise;
    const promise = this.#refreshUnderLock(loginId).finally(() => {
      if (this.#refreshing?.promise === promise) this.#refreshing = undefined;
    });
    this.#refreshing = { loginId, promise };
    return promise;
  }

  /**
   * Refreshes the session loginId: the tab's when the refresh was asked for, which may not be the tab's now. A
   * browser that will not write the lease or the refreshed record (SessionStorageError) cannot keep the
   * session: it is logged out with the newest refresh token the tab has, and the tab signs out.
   */
  async #refreshUnderLock(loginId: string | undefined): Promise<string | undefined> {
    if (this.deps.now() < this.#retryAt) throw new SessionUnavailableError(this.#retryAt);
    let newest: string | undefined;
    try {
      return await this.#refreshLocked(loginId, (token) => {
        newest = token;
      });
    } catch (error) {
      if (error instanceof SessionStorageError) await this.#abandon(loginId, newest);
      throw error;
    }
  }

  /** The refresh under the lock; seen gets each refresh token of loginId's session the tab has, the newest last. */
  #refreshLocked(loginId: string | undefined, seen: (token: string) => void): Promise<string | undefined> {
    return this.deps.lock.run(async () => {
      // Read under the lock: another tab may have refreshed, signed out or signed in meanwhile. A record of
      // another session is followed, never refreshed, even when the tab has heard of it while this refresh
      // waited for the lock: the caller's request was made in loginId's session, not in that one.
      const record = this.#read();
      if (!isRecordOf(record, loginId)) this.#follow(record);
      seen(record.refresh_token);
      const outcome = await this.#call("/api/v0/auth/refresh", record.refresh_token);
      // Read again before writing: without navigator.locks another tab can sign in while the lease is held.
      const now = this.#read();
      if (now?.login_id !== record.login_id) this.#follow(now);
      switch (outcome.kind) {
        case "tokens":
          seen(outcome.tokens.refresh_token);
          this.deps.storage.setItem(
            AUTH_KEY,
            JSON.stringify({ refresh_token: outcome.tokens.refresh_token, login_id: record.login_id })
          );
          this.#keep(outcome.tokens, outcome.receivedAt);
          if (this.#state.status !== "signed-in") this.#set({ status: "signed-in", loginId: record.login_id });
          return outcome.tokens.access_token;
        case "unauthorized":
          // Only a 401 to a refresh ends the session.
          this.#forget();
          this.#signedOut();
          return undefined;
        case "unavailable":
          throw new SessionUnavailableError(this.#backOff(outcome.retryAfterMs));
      }
    });
  }

  /** POSTs refresh_token to path, with its own timeout; never throws. A logout's answer does not matter. */
  async #call(path: "/api/v0/auth/refresh" | "/api/v0/auth/logout", refreshToken: string): Promise<Outcome> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
    try {
      const { data, response } = await this.deps.client.POST(path, {
        body: { refresh_token: refreshToken },
        signal: controller.signal,
      });
      if (data !== undefined) return { kind: "tokens", tokens: data, receivedAt: this.deps.now() };
      if (response.status === 401) return { kind: "unauthorized" };
      const retryAfter = Number(response.headers.get("Retry-After"));
      return { kind: "unavailable", retryAfterMs: retryAfter > 0 ? retryAfter * 1000 : undefined };
    } catch {
      // No network, or the timeout.
      return { kind: "unavailable" };
    } finally {
      clearTimeout(timer);
    }
  }

  /** Waits Retry-After, else 1, 2, 4 … up to 30 s after each failure in a row; returns when to try again. */
  #backOff(retryAfterMs: number | undefined): number {
    const delay = retryAfterMs ?? Math.min(1000 * 2 ** this.#failures, MAX_BACKOFF_MS);
    this.#failures++;
    this.#retryAt = this.deps.now() + delay;
    return this.#retryAt;
  }

  #keep(tokens: AuthTokens, receivedAt: number): void {
    this.#access = { token: tokens.access_token, expiresAt: receivedAt + tokens.access_token_expires_in * 1000 };
    this.#failures = 0;
    this.#retryAt = 0;
  }

  /**
   * Ends the session loginId, under the lock: removes the record and signs the tab out when the record is still
   * that session's, else follows the record. Returns whether it ended that session.
   */
  #end(loginId: string | undefined): boolean {
    const record = this.#read();
    if (!isRecordOf(record, loginId)) {
      this.#switchTo(record);
      return false;
    }
    this.#forget();
    this.#signedOut();
    return true;
  }

  /**
   * Ends the session loginId, which this browser cannot keep: logs it out with token, the newest refresh token
   * the tab has of it (the record's when the lease could not be taken), best effort; then removes the record if
   * the browser lets it and signs the tab out. A record of another session by then is followed instead.
   */
  async #abandon(loginId: string | undefined, token: string | undefined): Promise<void> {
    const record = this.#read();
    if (!isRecordOf(record, loginId)) {
      this.#switchTo(record);
      return;
    }
    await this.#call("/api/v0/auth/logout", token ?? record.refresh_token);
    this.#forget();
    this.#signedOut();
  }

  /** Removes the record; a browser that will not leaves it, and its refresh token then answers 401. */
  #forget(): void {
    try {
      this.deps.storage.removeItem(AUTH_KEY);
    } catch {
      // The session is over, or its refresh is answered 401 next time: a record that stays only ends it again.
    }
  }

  /** Follows another tab's change of the record, and stops the request that was meant for the old session. */
  #follow(record: AuthRecord | undefined): never {
    this.#switchTo(record);
    throw new SessionChangedError();
  }

  /**
   * Signed out when the record is gone, else the record's session, whose access token comes by a refresh. The
   * record of the session the tab is in already changes nothing: its access token stays, and so does its
   * state, which that session's refreshes decide (a starting or unavailable session keeps its retry).
   */
  #switchTo(record: AuthRecord | undefined): void {
    if (record === undefined) {
      this.#signedOut();
      return;
    }
    if (record.login_id === this.#state.loginId) return;
    this.#leaveSession();
    this.#set({ status: "signed-in", loginId: record.login_id });
  }

  #signedOut(): void {
    this.#leaveSession();
    this.#set({ status: "signed-out" });
  }

  /** Forgets what belonged to the session the tab leaves: its access token and its back-off. */
  #leaveSession(): void {
    this.#access = undefined;
    this.#failures = 0;
    this.#retryAt = 0;
  }

  #set(state: SessionState): void {
    if (state.status !== "unavailable") clearTimeout(this.#retryTimer);
    this.#state = state;
    for (const listener of this.#listeners) listener();
  }

  #read(): AuthRecord | undefined {
    return parse(this.deps.storage.getItem(AUTH_KEY));
  }
}

/** Whether record is the record of the session loginId. */
function isRecordOf(record: AuthRecord | undefined, loginId: string | undefined): record is AuthRecord {
  return record !== undefined && record.login_id === loginId;
}

function parse(value: string | null): AuthRecord | undefined {
  try {
    const record = JSON.parse(value ?? "null") as Partial<AuthRecord> | null;
    return typeof record?.refresh_token === "string" && typeof record.login_id === "string"
      ? { refresh_token: record.refresh_token, login_id: record.login_id }
      : undefined;
  } catch {
    return undefined;
  }
}
