// The lock the tabs of one browser share to refresh and to write the stored session one at a time (M1/P5
// design 3.2): two tabs refreshing the same refresh token at once would look like a stolen token, and a
// tab writing an old session back could overwrite a newer sign-in.

/** Runs a task while no other tab of this browser, and no other task of this tab, holds the lock. */
export interface RefreshLock {
  run<T>(task: () => Promise<T>): Promise<T>;
}

/** The name of the Web Locks lock. */
export const LOCK_NAME = "nwiki.auth.refresh";

/**
 * navigator.locks, which only secure contexts (HTTPS, localhost) have. The browser queues the tasks of
 * every tab, and a frozen tab keeps the lock until it thaws or is discarded.
 */
export function webLock(locks: Pick<LockManager, "request">): RefreshLock {
  return { run: (task) => locks.request(LOCK_NAME, task) };
}

/** The localStorage key of the lease. */
export const LEASE_KEY = "nwiki.auth.lease";
/** How long a lease lasts: longer than a refresh may take (8 s, M1/P2 design 3.5). */
const LEASE_MS = 10_000;
/** How long a tab waits after writing its lease before it reads it back. */
const LEASE_SETTLE_MS = 100;
/** How often a waiting tab looks at the lease, besides the storage events. */
const LEASE_POLL_MS = 200;

type Lease = { owner: string; expires: number };

export type LeaseDeps = {
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">;
  /** Calls the listener with the key of every storage event from another tab; returns the unsubscribe. */
  onStorage: (listener: (key: string | null) => void) => () => void;
  now: () => number;
  /** This tab's id: random, so it is unique without crypto.randomUUID, which non-secure contexts lack. */
  tabId: string;
};

/**
 * A lease in localStorage, for a page without navigator.locks: plain HTTP on a LAN address. A tab takes
 * the lease when it is free, expired or its own, and holds it after reading it back LEASE_SETTLE_MS later;
 * the others wait for a storage event or poll. localStorage has no compare-and-set, so two tabs taking the
 * lease at the same moment is made unlikely, not impossible (M1/P5 design 3.2).
 */
export function leaseLock(deps: LeaseDeps): RefreshLock {
  // The lease is the tab's, so a second task of the same tab would pass it: the tab's tasks queue first.
  let queue: Promise<unknown> = Promise.resolve();
  return {
    run<T>(task: () => Promise<T>): Promise<T> {
      const turn = queue.then(async () => {
        await acquire(deps);
        try {
          return await task();
        } finally {
          release(deps);
        }
      });
      queue = turn.catch(() => undefined);
      return turn;
    },
  };
}

function readLease(deps: LeaseDeps): Lease | undefined {
  try {
    const lease = JSON.parse(deps.storage.getItem(LEASE_KEY) ?? "null") as Partial<Lease> | null;
    return typeof lease?.owner === "string" && typeof lease.expires === "number"
      ? { owner: lease.owner, expires: lease.expires }
      : undefined;
  } catch {
    return undefined;
  }
}

/** Takes the lease: at once when no other tab holds it, else once the holder lets it go or it expires. */
async function acquire(deps: LeaseDeps): Promise<void> {
  if (await take(deps)) return;
  await changeOrPoll(deps);
  return acquire(deps);
}

/** Writes this tab's lease unless another tab holds one, and tells whether it is still this tab's later. */
async function take(deps: LeaseDeps): Promise<boolean> {
  const lease = readLease(deps);
  if (lease !== undefined && lease.expires > deps.now() && lease.owner !== deps.tabId) return false;
  deps.storage.setItem(LEASE_KEY, JSON.stringify({ owner: deps.tabId, expires: deps.now() + LEASE_MS }));
  await sleep(LEASE_SETTLE_MS);
  return readLease(deps)?.owner === deps.tabId;
}

function release(deps: LeaseDeps): void {
  if (readLease(deps)?.owner === deps.tabId) deps.storage.removeItem(LEASE_KEY);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Resolves on the next storage event about the lease, or after LEASE_POLL_MS. */
function changeOrPoll(deps: LeaseDeps): Promise<void> {
  let unsubscribe: (() => void) | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  return new Promise<void>((resolve) => {
    unsubscribe = deps.onStorage((key) => {
      if (key === LEASE_KEY || key === null) resolve();
    });
    timer = setTimeout(resolve, LEASE_POLL_MS);
  }).finally(() => {
    clearTimeout(timer);
    unsubscribe?.();
  });
}
