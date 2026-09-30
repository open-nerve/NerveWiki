// Test doubles of the browser for the auth tests: one localStorage shared by several tabs, which tells
// the other tabs about every change with a storage event, and a lock whose holder the tests can see. The
// event comes in a microtask of the writing task, sooner than in a browser, where it reaches the other
// tabs in a later task, maybe after later writes and lock grants: a test plays that with hold() and
// deliver(). The event carries the changed key only; the code under test reads the value itself.

import type { RefreshLock } from "../refresh-lock";

type StorageListener = (key: string | null) => void;

/** The localStorage of one browser: every tab's view writes the same data. */
export class SharedStorage {
  readonly data = new Map<string, string>();
  private readonly listeners = new Map<string, Set<StorageListener>>();
  /** The events hold() keeps back, in the order of their writes; undefined while events flow. */
  private held: (() => void)[] | undefined;

  /** The view of the tab `tab`: its writes reach the other tabs' listeners, never its own. */
  tab(tab: string) {
    return {
      getItem: (key: string) => this.data.get(key) ?? null,
      setItem: (key: string, value: string) => this.change(tab, key, String(value)),
      removeItem: (key: string) => this.change(tab, key, null),
      /** Subscribes this tab to the storage events of the other tabs; returns the unsubscribe. */
      onStorage: (listener: StorageListener) => {
        const set = this.listeners.get(tab) ?? new Set<StorageListener>();
        this.listeners.set(tab, set);
        set.add(listener);
        return () => {
          set.delete(listener);
        };
      },
    };
  }

  /** A write by a tab outside the tests' tabs, e.g. a test playing another tab by hand. */
  write(key: string, value: string | null): void {
    this.change("elsewhere", key, value);
  }

  /** Keeps every storage event back until deliver(), as a browser may deliver one after later writes. */
  hold(): void {
    this.held ??= [];
  }

  /** Delivers the held events in the order of their writes; the events after it flow again. */
  deliver(): void {
    const held = this.held ?? [];
    this.held = undefined;
    for (const event of held) event();
  }

  /** Sets key to value, or removes it for null; like a browser, tells no tab of a write that changes nothing. */
  private change(writer: string, key: string, value: string | null): void {
    if ((this.data.get(key) ?? null) === value) return;
    if (value === null) this.data.delete(key);
    else this.data.set(key, value);
    this.notify(writer, key);
  }

  private notify(writer: string, key: string): void {
    for (const [tab, set] of this.listeners) {
      if (tab === writer) continue;
      const event = () => {
        for (const listener of set) listener(key);
      };
      // Like a browser, never inside the write; unlike one, before the writing task ends (a microtask),
      // unless the test holds the events back.
      if (this.held === undefined) queueMicrotask(event);
      else this.held.push(event);
    }
  }
}

/** A lock that runs one task at a time, in order, and tells whether a task holds it now. */
export class RecordingLock implements RefreshLock {
  held = false;
  private queue: Promise<unknown> = Promise.resolve();

  run<T>(task: () => Promise<T>): Promise<T> {
    const turn = this.queue.then(async () => {
      this.held = true;
      try {
        return await task();
      } finally {
        this.held = false;
      }
    });
    this.queue = turn.catch(() => undefined);
    return turn;
  }
}

/** A promise the test resolves or rejects by hand. */
export function gate<T = void>() {
  let open!: (value: T) => void;
  let fail!: (reason: unknown) => void;
  const promise = new Promise<T>((resolve, reject) => {
    open = resolve;
    fail = reject;
  });
  return { promise, open, fail };
}
