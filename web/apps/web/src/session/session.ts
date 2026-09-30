import { createClient, type ApiClient } from "@nervewiki/api-client";

import { authMiddleware } from "./auth-middleware";
import { leaseLock, webLock } from "./refresh-lock";
import { AUTH_KEY, TokenManager } from "./token-manager";

// The web app's session (M1/P5 design 3.2): the only module that creates clients of the API. main.tsx
// builds one Session for the page and starts it; nothing here runs on import.

export type SessionDeps = {
  /** Where the record lives: localStorage, or memory where the browser blocks site data. */
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">;
  /** Calls the listener with the key of every change another tab makes to storage; returns the unsubscribe. */
  onStorage: (listener: (key: string | null) => void) => () => void;
  /** navigator.locks where the page has it (HTTPS, localhost); without it, as on plain HTTP at a LAN address, a lease in storage. */
  locks: Pick<LockManager, "request"> | undefined;
  now: () => number;
  /** n random bytes as hexadecimal. */
  randomHex: (bytes: number) => string;
  /** The clients' options: the page's own origin and the browser's fetch unless a test says otherwise. */
  client?: Parameters<typeof createClient>[0];
};

export class Session {
  /**
   * The client of the operations that need no token: the instance, sign-in and sign-up, and the token
   * manager's own refresh and logout. Sign-in answers 401 for a wrong password: it must not go through the
   * middleware, which would take that for the end of the session.
   */
  readonly public: ApiClient;
  readonly tokens: TokenManager;
  #unsubscribe: (() => void) | undefined;
  #started: Promise<void> | undefined;

  constructor(private readonly deps: SessionDeps) {
    this.public = createClient(deps.client);
    const lock = deps.locks
      ? webLock(deps.locks)
      : leaseLock({ storage: deps.storage, onStorage: deps.onStorage, now: deps.now, tabId: deps.randomHex(16) });
    this.tokens = new TokenManager({
      storage: deps.storage,
      lock,
      client: this.public,
      now: deps.now,
      randomHex: deps.randomHex,
    });
  }

  /**
   * The client of every other operation for the stores of the session loginId: the access token on each
   * request, a 401 renewed once, and nothing sent once the tab is in another session (SessionChangedError).
   */
  clientFor(loginId: string): ApiClient {
    const api = createClient(this.deps.client);
    api.use(authMiddleware(this.tokens, loginId));
    return api;
  }

  /**
   * Follows the other tabs' changes of the record and decides the session, once: without a record the tab
   * is signed out and asks nothing; with one, the first refresh decides.
   */
  start(): Promise<void> {
    this.#started ??= this.#start();
    return this.#started;
  }

  /** Stops following the other tabs (for tests: a page's session lives as long as the page). */
  dispose(): void {
    this.#unsubscribe?.();
  }

  #start(): Promise<void> {
    this.#unsubscribe = this.deps.onStorage((key) => {
      if (key === AUTH_KEY || key === null) this.tokens.handleStorageChange();
    });
    return this.tokens.start();
  }
}

/** The session's dependencies in this tab of the browser, with the record in storage. */
export function browserSessionDeps(storage: SessionDeps["storage"]): SessionDeps {
  return {
    storage,
    // Only the events of the storage the record is in: none for the memory fallback, which no tab shares.
    onStorage: (listener) => {
      const handle = (event: StorageEvent) => {
        if (event.storageArea === storage) listener(event.key);
      };
      window.addEventListener("storage", handle);
      return () => window.removeEventListener("storage", handle);
    },
    locks: "locks" in navigator ? navigator.locks : undefined,
    now: Date.now,
    randomHex,
  };
}

/** n random bytes as hexadecimal, from crypto.getRandomValues, which pages on plain HTTP have too. */
function randomHex(bytes: number): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(bytes)), (b) => b.toString(16).padStart(2, "0")).join("");
}
