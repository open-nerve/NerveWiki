import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SharedStorage, gate } from "./testing/fake-browser";
import { LEASE_KEY, LOCK_NAME, leaseLock, webLock } from "./refresh-lock";

// The lease (M1/P5 design 3.2) with fake timers: every wait is a timer the test advances, never a sleep.

function tab(storage: SharedStorage, tabId: string) {
  const view = storage.tab(tabId);
  return leaseLock({ storage: view, onStorage: (listener) => view.onStorage(listener), now: () => Date.now(), tabId });
}

/** Runs a task that records when it starts and ends and waits on a gate the test opens. */
function task(log: string[], name: string) {
  const g = gate<string>();
  const run = async () => {
    log.push(`${name} start ${Date.now()}`);
    const value = await g.promise;
    log.push(`${name} end ${Date.now()}`);
    return value;
  };
  return { run, finish: () => g.open(name) };
}

const lease = (storage: SharedStorage) => JSON.parse(storage.data.get(LEASE_KEY) ?? "null") as unknown;

describe("leaseLock", () => {
  beforeEach(() => {
    vi.useFakeTimers({ now: 0 });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("takes a free lease once it reads it back, and deletes it after the task", async () => {
    const storage = new SharedStorage();
    const log: string[] = [];
    const a = task(log, "a");
    const done = tab(storage, "A").run(a.run);

    await vi.advanceTimersByTimeAsync(99);
    expect(log).toEqual([]);
    expect(LEASE_KEY).toBe("nwiki.auth.lease");
    expect(lease(storage)).toEqual({ owner: "A", expires: 10_000 });
    await vi.advanceTimersByTimeAsync(1);
    expect(log).toEqual(["a start 100"]);

    a.finish();
    await expect(done).resolves.toBe("a");
    expect(storage.data.has(LEASE_KEY)).toBe(false);
  });

  it("makes another tab wait while it holds the lease, and wakes it on the release", async () => {
    const storage = new SharedStorage();
    const log: string[] = [];
    const [a, b] = [task(log, "a"), task(log, "b")];
    const first = tab(storage, "A").run(a.run);
    await vi.advanceTimersByTimeAsync(100);
    const second = tab(storage, "B").run(b.run);

    await vi.advanceTimersByTimeAsync(5_000);
    expect(log).toEqual(["a start 100"]);

    // The release is a storage event for B, which takes the lease at once instead of at its next poll.
    await vi.advanceTimersByTimeAsync(50);
    a.finish();
    await first;
    await vi.advanceTimersByTimeAsync(100);
    expect(log).toEqual(["a start 100", "a end 5150", "b start 5250"]);
    b.finish();
    await second;
  });

  it("finds a released lease by polling when no storage event comes", async () => {
    const storage = new SharedStorage();
    const log: string[] = [];
    const b = task(log, "b");
    // A tab outside the test holds the lease; it expires in 10 s but goes away sooner, silently.
    storage.data.set(LEASE_KEY, JSON.stringify({ owner: "A", expires: 10_000 }));
    const waiting = tab(storage, "B").run(b.run);

    await vi.advanceTimersByTimeAsync(1_250);
    storage.data.delete(LEASE_KEY);
    // The next poll, at 1400 ms, finds it free; B takes it and starts 100 ms later.
    await vi.advanceTimersByTimeAsync(249);
    expect(log).toEqual([]);
    await vi.advanceTimersByTimeAsync(1);
    expect(log).toEqual(["b start 1500"]);
    b.finish();
    await waiting;
  });

  it("takes over an expired lease", async () => {
    const storage = new SharedStorage();
    const log: string[] = [];
    // A tab took the lease and was frozen: its lease is never released.
    storage.data.set(LEASE_KEY, JSON.stringify({ owner: "A", expires: 10_000 }));
    const b = task(log, "b");
    const waiting = tab(storage, "B").run(b.run);

    await vi.advanceTimersByTimeAsync(10_000 - 1);
    expect(log).toEqual([]);
    await vi.advanceTimersByTimeAsync(200 + 100);
    expect(log).toHaveLength(1);
    expect(lease(storage)).toMatchObject({ owner: "B" });
    b.finish();
    await waiting;
  });

  it("deletes only its own lease", async () => {
    const storage = new SharedStorage();
    const log: string[] = [];
    const [a, b] = [task(log, "a"), task(log, "b")];
    const first = tab(storage, "A").run(a.run);
    await vi.advanceTimersByTimeAsync(100);
    // A holds the lease longer than it lasts; B takes it over.
    const second = tab(storage, "B").run(b.run);
    await vi.advanceTimersByTimeAsync(10_000 + 300);
    expect(lease(storage)).toMatchObject({ owner: "B" });

    a.finish();
    await first;
    expect(lease(storage)).toMatchObject({ owner: "B" });
    b.finish();
    await second;
    expect(storage.data.has(LEASE_KEY)).toBe(false);
  });

  it("waits when another tab wrote its lease over this tab's before the read-back", async () => {
    const storage = new SharedStorage();
    const log: string[] = [];
    const a = task(log, "a");
    const waiting = tab(storage, "A").run(a.run);

    // Another tab saw the lease free at the same moment and wrote its own after A's.
    await vi.advanceTimersByTimeAsync(50);
    storage.write(LEASE_KEY, JSON.stringify({ owner: "B", expires: 50 + 10_000 }));
    await vi.advanceTimersByTimeAsync(3_000);
    expect(log).toEqual([]);
    expect(lease(storage)).toMatchObject({ owner: "B" });

    storage.write(LEASE_KEY, null);
    await vi.advanceTimersByTimeAsync(100);
    expect(log).toEqual(["a start 3150"]);
    a.finish();
    await waiting;
  });

  it("runs the tasks of one tab one after the other", async () => {
    const storage = new SharedStorage();
    const lock = tab(storage, "A");
    const log: string[] = [];
    const [a, b] = [task(log, "a"), task(log, "b")];
    const first = lock.run(a.run);
    const second = lock.run(b.run);

    await vi.advanceTimersByTimeAsync(1_000);
    expect(log).toEqual(["a start 100"]);
    a.finish();
    await first;
    await vi.advanceTimersByTimeAsync(100);
    expect(log).toEqual(["a start 100", "a end 1000", "b start 1100"]);
    b.finish();
    await second;
  });

  it("releases the lease when the task fails, and passes the failure on", async () => {
    const storage = new SharedStorage();
    const lock = tab(storage, "A");
    const failed = lock.run(async () => {
      throw new Error("refresh failed");
    });
    const caught = expect(failed).rejects.toThrow("refresh failed");
    await vi.advanceTimersByTimeAsync(100);
    await caught;
    expect(storage.data.has(LEASE_KEY)).toBe(false);

    const next = lock.run(async () => "next");
    await vi.advanceTimersByTimeAsync(100);
    await expect(next).resolves.toBe("next");
  });
});

describe("webLock", () => {
  it("runs the task under the lock nwiki.auth.refresh and passes its result on", async () => {
    // A lock the test grants by hand, held from the grant until the callback it runs settles.
    const requests: unknown[][] = [];
    const grant = gate();
    let held = false;
    const locks = {
      request: (async (...args: unknown[]) => {
        const granted = args.pop() as () => Promise<unknown>;
        requests.push(args);
        await grant.promise;
        held = true;
        try {
          return await granted();
        } finally {
          held = false;
        }
      }) as LockManager["request"],
    };
    const body = gate<number>();
    const seen: boolean[] = [];
    const done = webLock(locks).run(async () => {
      seen.push(held);
      const value = await body.promise;
      seen.push(held);
      return value;
    });

    // One exclusive request (no options), and the task waits for the grant.
    expect(requests).toEqual([[LOCK_NAME]]);
    expect(seen).toEqual([]);
    grant.open();
    body.open(42);
    await expect(done).resolves.toBe(42);
    expect(seen).toEqual([true, true]);
    expect(held).toBe(false);
    expect(LOCK_NAME).toBe("nwiki.auth.refresh");
  });
});
