import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { SharedStorage } from "../session/testing/fake-browser";
import { leaseLeadership, webLockLeadership, type Leadership } from "./leadership";
import { FakeLocks } from "./testing/fake-locks";

const NAME = "nwiki.events.login-0";
const TTL = 60_000;

/** A tab taking part in an election: whether it holds, how often it led, and how to stop it. */
type Tab = { leadership: Leadership; holding: boolean; leads: number; stop: () => void };

/** One browser's tabs, which elect with Web Locks or with a lease in storage. */
function browserOf(kind: "Web Locks" | "the lease") {
  const locks = new FakeLocks();
  const storage = new SharedStorage();
  return Object.assign(
    (id: string): Tab => {
      const view = storage.tab(id);
      const leadership =
        kind === "Web Locks"
          ? webLockLeadership(locks, NAME)
          : leaseLeadership({ storage: view, onStorage: view.onStorage, now: () => Date.now(), tabId: id }, NAME);
      const stopped = new AbortController();
      const tab: Tab = { leadership, holding: false, leads: 0, stop: () => stopped.abort() };
      void leadership.run(async (lost) => {
        tab.holding = true;
        tab.leads += 1;
        await new Promise((resolve) => lost.addEventListener("abort", resolve, { once: true }));
        tab.holding = false;
      }, stopped.signal);
      return tab;
    },
    { storage }
  );
}

/** Lets the elections' timers and promises run for ms. */
async function settle(ms = 100): Promise<void> {
  await vi.advanceTimersByTimeAsync(ms);
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});

afterEach(() => {
  vi.useRealTimers();
});

describe.each(["Web Locks", "the lease"] as const)("the election with %s", (kind) => {
  test("one tab of several holds; once it stops, another holds", async () => {
    const tab = browserOf(kind);
    const [a, b, c] = [tab("a"), tab("b"), tab("c")];
    await settle();
    expect([a, b, c].filter((t) => t.holding)).toHaveLength(1);
    const holder = [a, b, c].find((t) => t.holding);

    holder?.stop();
    await settle(TTL);

    expect(holder?.holding).toBe(false);
    expect([a, b, c].filter((t) => t.holding)).toHaveLength(1);
  });

  test("a tab that steals holds at once; the one it took the lead from loses it and holds again later", async () => {
    const tab = browserOf(kind);
    const a = tab("a");
    await settle();
    const b = tab("b");
    await settle();
    expect([a.holding, b.holding]).toEqual([true, false]);

    b.leadership.steal();
    await settle();

    expect([a.holding, b.holding]).toEqual([false, true]);
    b.stop();
    await settle(TTL);
    expect([a.holding, a.leads]).toEqual([true, 2]);
  });

  test("a tab that yields lets the lead go and stays out until it rejoins", async () => {
    const tab = browserOf(kind);
    const a = tab("a");
    await settle();
    const b = tab("b");
    await settle();

    a.leadership.yield();
    await settle(TTL);

    expect([a.holding, b.holding]).toEqual([false, true]);
    b.stop();
    await settle(TTL);
    expect(a.holding).toBe(false);
    a.leadership.rejoin();
    await settle(TTL);
    expect(a.holding).toBe(true);
  });

  test("a steal asked while yielded is forgotten: rejoined, the tab waits its turn", async () => {
    const tab = browserOf(kind);
    const a = tab("a");
    await settle();
    const b = tab("b");
    await settle();

    b.leadership.yield();
    b.leadership.steal();
    b.leadership.rejoin();
    await settle();

    expect([a.holding, b.holding]).toEqual([true, false]);
  });

  test("a stopped tab leads no more", async () => {
    const tab = browserOf(kind);
    const a = tab("a");
    await settle();

    a.stop();
    await settle(TTL);

    expect([a.holding, a.leads]).toEqual([false, 1]);
  });
});

describe("the election with the lease", () => {
  test("of tabs that take a free lease at once, only one holds", async () => {
    const tab = browserOf("the lease");

    const tabs = [tab("a"), tab("b"), tab("c")];
    await settle();

    expect(tabs.filter((t) => t.holding)).toHaveLength(1);
  });

  test("a tab whose lease another tab wrote over while it settled does not hold", async () => {
    const tab = browserOf("the lease");
    const a = tab("a");

    // Another tab, which read the lease free as a did, writes it a moment later.
    tab.storage.write(NAME, JSON.stringify({ tab: "z", until: Date.now() + TTL }));
    await settle();

    expect([a.holding, a.leads]).toEqual([false, 0]);
  });

  test("a steal asked while the tab settled is answered by its holding: once another takes over, it waits its turn", async () => {
    const tab = browserOf("the lease");
    const b = tab("b");
    b.leadership.steal();
    await settle();
    expect(b.holding).toBe(true);
    const c = tab("c");
    await settle();

    c.leadership.steal();
    await settle();

    expect([b.holding, c.holding]).toEqual([false, true]);
  });

  test("a steal asked while the tab settled, which another tab won, is answered too: the winner's renewal does not wake it to steal", async () => {
    const tab = browserOf("the lease");
    const b = tab("b");
    // Another tab, which read the lease free as b did, writes it a moment later; b is asked to steal meanwhile.
    tab.storage.write(NAME, JSON.stringify({ tab: "z", until: Date.now() + TTL }));
    b.leadership.steal();
    await settle();
    expect(b.holding).toBe(false);

    // The winner renews.
    tab.storage.write(NAME, JSON.stringify({ tab: "z", until: Date.now() + TTL }));
    await settle();

    expect(b.holding).toBe(false);
  });

  test("a tab whose storage refuses the lease does not hold, and takes it once the storage takes writes again", async () => {
    const storage = new SharedStorage();
    const view = storage.tab("a");
    let full = true;
    const refusing = {
      ...view,
      setItem: (key: string, value: string) => {
        if (full) {
          throw new DOMException("The quota has been exceeded.", "QuotaExceededError");
        }
        view.setItem(key, value);
      },
    };
    let holding = false;
    const a = leaseLeadership(
      { storage: refusing, onStorage: view.onStorage, now: () => Date.now(), tabId: "a" },
      NAME
    );
    void a.run(async (lost) => {
      holding = true;
      await new Promise((resolve) => lost.addEventListener("abort", resolve, { once: true }));
    }, new AbortController().signal);
    await settle();
    expect(holding).toBe(false);

    full = false;
    await settle(TTL);

    expect(holding).toBe(true);
  });

  test.each(["yields", "stops"])("a holder that %s lets the lease go at once, before its lead returns", async (how) => {
    const tab = browserOf("the lease");
    const view = tab.storage.tab("a");
    const a = leaseLeadership({ storage: view, onStorage: view.onStorage, now: () => Date.now(), tabId: "a" }, NAME);
    const stopped = new AbortController();
    // A page that goes away may not live to see its lead return.
    void a.run(() => new Promise(() => undefined), stopped.signal);
    await settle();
    const b = tab("b");
    await settle();
    expect(b.holding).toBe(false);

    if (how === "yields") {
      a.yield();
    } else {
      stopped.abort();
    }
    await settle();

    expect(b.holding).toBe(true);
  });

  test("a holder that renews keeps the lease; a frozen one loses it once it runs out, and learns so at its next renewal", async () => {
    const tab = browserOf("the lease");
    const a = tab("a");
    await settle();
    const b = tab("b");
    for (let i = 0; i < 5; i++) {
      a.leadership.renew(TTL);
      // oxlint-disable-next-line no-await-in-loop -- one renewal after another
      await settle(TTL / 2);
    }
    expect([a.holding, a.leads, b.leads]).toEqual([true, 1, 0]);

    // A frozen tab hears nothing of the storage until it thaws.
    tab.storage.hold();
    await settle(TTL);
    expect([a.holding, b.holding]).toEqual([true, true]);
    a.leadership.renew(TTL);
    await settle();

    expect([a.holding, b.holding]).toEqual([false, true]);
    tab.storage.deliver();
  });
});
