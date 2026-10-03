import type { Locks } from "../leadership";

// The Web Locks of one browser for the election's tests, as the spec has
// them: requests of a name line up and are granted one after another; a
// request with steal is granted at once and the holder's request fails with
// an AbortError, its callback left running; a waiting request's signal takes
// it out of the line, failing with an AbortError.

type Waiter = { grant: () => void; drop: () => void };

export class FakeLocks implements Locks {
  readonly #held = new Map<string, object>();
  readonly #lines = new Map<string, Waiter[]>();

  /** Whether a request of name holds the lock. */
  held(name: string): boolean {
    return this.#held.has(name);
  }

  request(name: string, options: LockOptions, callback: () => Promise<void>): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const grant = () => {
        const holder = {};
        this.#held.set(name, holder);
        stolen.set(holder, reject);
        const release = () => {
          if (this.#held.get(name) === holder) {
            this.#held.delete(name);
            this.#next(name);
          }
        };
        void (async () => {
          try {
            await Promise.resolve();
            await callback();
            release();
            resolve();
          } catch (error) {
            release();
            reject(error);
          }
        })();
      };
      if (options.steal) {
        const holder = this.#held.get(name);
        if (holder) {
          this.#held.delete(name);
          stolen.get(holder)?.(new DOMException("The lock was stolen.", "AbortError"));
        }
        grant();
        return;
      }
      if (options.signal?.aborted) {
        reject(new DOMException("The request was aborted.", "AbortError"));
        return;
      }
      const line = this.#lines.get(name) ?? [];
      this.#lines.set(name, line);
      if (!this.#held.has(name) && line.length === 0) {
        grant();
        return;
      }
      const waiter: Waiter = {
        grant,
        drop: () => reject(new DOMException("The request was aborted.", "AbortError")),
      };
      line.push(waiter);
      options.signal?.addEventListener(
        "abort",
        () => {
          const at = line.indexOf(waiter);
          if (at >= 0) {
            line.splice(at, 1);
            waiter.drop();
          }
        },
        { once: true }
      );
    });
  }

  #next(name: string): void {
    this.#lines.get(name)?.shift()?.grant();
  }
}

const stolen = new WeakMap<object, (error: unknown) => void>();
