// Re-reading a page's content as events come (M5 design 4.11): while
// someone edits, every autosave is an event, and a tab that re-read at once
// would have the server parse the page every two seconds for each reader.
// A visible tab reads a key at most once in its interval, the last request
// of the interval; a hidden tab reads once it is visible again. A key's
// interval is the one its last request named, INTERVAL_MS unless it named
// another (a notebook's tree): a read already scheduled keeps its time, so
// each key is asked for at one interval.

/** How often a visible tab reads the same key at most, unless its request says otherwise. */
export const INTERVAL_MS = 5_000;

/** Whether the page is visible, and its changes. */
export type Visibility = {
  visible(): boolean;
  onChange(listener: () => void): () => void;
};

export class Refresher {
  readonly #last = new Map<string, number>();
  readonly #intervals = new Map<string, number>();
  readonly #due = new Map<string, () => void>();
  readonly #timers = new Map<string, ReturnType<typeof setTimeout>>();
  readonly #unsubscribe: () => void;

  constructor(
    private readonly page: Visibility,
    private readonly now: () => number
  ) {
    this.#unsubscribe = page.onChange(() => {
      if (page.visible()) {
        // #schedule may read, which takes the key out of #due.
        for (const key of Array.from(this.#due.keys())) {
          this.#schedule(key);
        }
      }
    });
  }

  /** request asks for read of key: now, at the end of the key's interval, or once the page is visible. */
  request(key: string, read: () => void, interval = INTERVAL_MS): void {
    this.#due.set(key, read);
    this.#intervals.set(key, interval);
    if (this.page.visible()) {
      this.#schedule(key);
    }
  }

  /** stop drops what is due and its timers. */
  stop(): void {
    this.#unsubscribe();
    for (const timer of this.#timers.values()) {
      clearTimeout(timer);
    }
    this.#timers.clear();
    this.#due.clear();
  }

  #schedule(key: string): void {
    if (this.#timers.has(key)) {
      return;
    }
    const wait = (this.#last.get(key) ?? -Infinity) + (this.#intervals.get(key) ?? INTERVAL_MS) - this.now();
    if (wait <= 0) {
      this.#read(key);
      return;
    }
    this.#timers.set(
      key,
      setTimeout(() => {
        this.#timers.delete(key);
        if (this.page.visible()) {
          this.#read(key);
        }
      }, wait)
    );
  }

  #read(key: string): void {
    const read = this.#due.get(key);
    this.#due.delete(key);
    if (read) {
      this.#last.set(key, this.now());
      read();
    }
  }
}
