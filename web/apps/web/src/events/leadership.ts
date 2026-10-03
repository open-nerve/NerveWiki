// Which tab of a login holds the event stream (M5 design 4.11): one stream
// per browser, since without TLS the browser keeps six connections per
// origin and a stream per tab would take them all. The tabs elect a holder
// with Web Locks where the page has them, and with a lease in storage where
// it has not (plain HTTP at a LAN address).

/**
 * Leadership is a tab's part in the election. run takes part until its
 * signal aborts. Each time the tab becomes the holder, lead runs with a
 * signal that aborts when the tab loses the lead (stolen, yielded, or the
 * run's end); the lead is let go once lead returns.
 */
export interface Leadership {
  run(lead: (lost: AbortSignal) => Promise<void>, signal: AbortSignal): Promise<void>;
  /** steal takes the lead from a holder that seems frozen, which does not let go of a Web Lock; not while yielded. */
  steal(): void;
  /** yield lets the lead go, as the page is hidden for good or frozen, and stays out until rejoin. */
  yield(): void;
  /** rejoin takes part again after yield, as a tab that waits its turn. */
  rejoin(): void;
  /** renew is the holder's sign of life, valid for ttl milliseconds: a lease's expiry. */
  renew(ttl: number): void;
}

/** The part of navigator.locks the election uses. */
export type Locks = {
  request(name: string, options: LockOptions, callback: () => Promise<void>): Promise<void>;
};

/**
 * webLockLeadership elects with the Web Lock name: the browser grants it to
 * one tab and to the next in line once it is let go, closed tabs included.
 * steal makes the next request take it at once; the tab it was taken from
 * sees its request fail and lines up again.
 */
export function webLockLeadership(locks: Locks, name: string): Leadership {
  let stealNext = false;
  let yielded = false;
  let waiting: AbortController | undefined;
  let holding: AbortController | undefined;
  let wake: (() => void) | undefined;
  const nudge = () => {
    waiting?.abort();
    wake?.();
  };
  const round = async (lead: (lost: AbortSignal) => Promise<void>, signal: AbortSignal) => {
    if (yielded) {
      await pause(Infinity, signal, (w) => (wake = w));
      return;
    }
    waiting = new AbortController();
    const options: LockOptions = stealNext ? { steal: true } : { signal: either(signal, waiting.signal) };
    stealNext = false;
    try {
      await locks.request(name, options, async () => {
        waiting = undefined;
        if (signal.aborted || yielded) {
          return;
        }
        holding = new AbortController();
        await lead(holding.signal);
      });
    } catch {
      // A request let go of while it waited, or a lock stolen from this tab.
    }
    holding?.abort();
    holding = undefined;
  };
  const stop = () => {
    holding?.abort();
    nudge();
  };
  return {
    async run(lead, signal) {
      signal.addEventListener("abort", stop, { once: true });
      while (!signal.aborted) {
        // oxlint-disable-next-line no-await-in-loop -- an election's rounds come one after another
        await round(lead, signal);
      }
      signal.removeEventListener("abort", stop);
    },
    steal() {
      if (!yielded) {
        stealNext = true;
        nudge();
      }
    },
    yield() {
      yielded = true;
      holding?.abort();
      nudge();
    },
    rejoin() {
      yielded = false;
      stealNext = false;
      wake?.();
    },
    renew() {},
  };
}

/** What a lease leadership needs: the storage the tabs share, its changes in other tabs, the clock and the tab's id. */
export type LeaseDeps = {
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">;
  onStorage: (listener: (key: string | null) => void) => () => void;
  now: () => number;
  tabId: string;
};

type Lease = { tab: string; until: number };

/** How long a tab waits after writing the lease before it reads it back: several tabs that write at once, the last wins. */
const SETTLE_MS = 50;

/** How long a lease lasts until the holder's first renewal says: three of the server's default heartbeats. */
const FIRST_TTL_MS = 60_000;

/**
 * leaseLeadership elects with a lease in storage under key: { tab, until }.
 * A tab takes it when it is absent or past its until (or to steal): it
 * writes itself, waits a moment, and holds it if it reads itself back. The
 * holder renews it on every frame, which background timers do not delay; a
 * frozen holder stops renewing, its lease runs out and another tab takes
 * it, and once it thaws its next renewal finds the lease not its own and
 * its lead lost. A holder that yields or stops removes the lease at once.
 * The others look again when the lease changes and when it should have run
 * out.
 */
export function leaseLeadership(deps: LeaseDeps, key: string): Leadership {
  let ttl = FIRST_TTL_MS;
  let stealNext = false;
  let yielded = false;
  let holding: AbortController | undefined;
  let wake: (() => void) | undefined;
  const read = (): Lease | undefined => {
    try {
      const lease = JSON.parse(deps.storage.getItem(key) ?? "null") as Lease | null;
      return lease && typeof lease.tab === "string" && typeof lease.until === "number" ? lease : undefined;
    } catch {
      return undefined;
    }
  };
  const mine = () => read()?.tab === deps.tabId;
  // A write the storage refuses (full, or blocked) is a lease not taken, or not renewed: it runs out.
  const write = (): boolean => {
    try {
      deps.storage.setItem(key, JSON.stringify({ tab: deps.tabId, until: deps.now() + ttl }));
      return true;
    } catch {
      return false;
    }
  };
  const release = () => {
    try {
      if (mine()) {
        deps.storage.removeItem(key);
      }
    } catch {
      // The lease runs out.
    }
  };
  // letGo lets the lease go at once, before the lead returns: a page hidden for good may not live to see it return.
  const letGo = () => {
    if (holding) {
      release();
      holding.abort();
    }
    wake?.();
  };
  const round = async (lead: (lost: AbortSignal) => Promise<void>, signal: AbortSignal) => {
    if (yielded) {
      await pause(Infinity, signal, (w) => (wake = w));
      return;
    }
    const lease = read();
    if (stealNext || lease === undefined || lease.until <= deps.now()) {
      stealNext = false;
      if (!write()) {
        // Looked at again once woken, or a lease later.
        await pause(ttl, signal, (w) => (wake = w));
        return;
      }
      await pause(SETTLE_MS, signal, () => undefined);
      if (signal.aborted || yielded) {
        release();
        return;
      }
      if (mine()) {
        // A steal asked for while this tab settled is answered: it holds.
        stealNext = false;
        holding = new AbortController();
        await lead(holding.signal);
        holding = undefined;
        release();
        return;
      }
    }
    const until = read()?.until ?? deps.now();
    await pause(Math.max(until - deps.now(), 0) + 1, signal, (w) => (wake = w));
  };
  return {
    async run(lead, signal) {
      const unsubscribe = deps.onStorage((changed) => {
        if (changed !== key && changed !== null) {
          return;
        }
        if (holding && !mine()) {
          holding.abort();
        }
        wake?.();
      });
      signal.addEventListener("abort", letGo, { once: true });
      while (!signal.aborted) {
        // oxlint-disable-next-line no-await-in-loop -- an election's rounds come one after another
        await round(lead, signal);
      }
      signal.removeEventListener("abort", letGo);
      unsubscribe();
    },
    steal() {
      if (!yielded) {
        stealNext = true;
        wake?.();
      }
    },
    yield() {
      yielded = true;
      letGo();
    },
    rejoin() {
      yielded = false;
      stealNext = false;
      wake?.();
    },
    renew(next) {
      ttl = next;
      if (!holding) {
        return;
      }
      if (mine()) {
        write();
      } else {
        holding.abort();
      }
    },
  };
}

/**
 * pause waits ms (Infinity: until woken), until signal aborts, or until the
 * function it hands to onWake is called, whichever comes first.
 */
function pause(ms: number, signal: AbortSignal, onWake: (wake: () => void) => void): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    let timer: ReturnType<typeof setTimeout> | undefined;
    const done = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", done);
      resolve();
    };
    if (Number.isFinite(ms)) {
      timer = setTimeout(done, ms);
    }
    signal.addEventListener("abort", done, { once: true });
    onWake(done);
  });
}

/** either aborts once a or b does. */
function either(a: AbortSignal, b: AbortSignal): AbortSignal {
  const both = new AbortController();
  for (const s of [a, b]) {
    if (s.aborted) {
      both.abort();
    }
    s.addEventListener("abort", () => both.abort(), { once: true });
  }
  return both.signal;
}
