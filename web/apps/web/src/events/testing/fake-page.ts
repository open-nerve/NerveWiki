import type { PageLifecycle } from "../hub";

// A tab's page for the tests: visible or hidden, and its lifecycle's events,
// which the test fires.

export class FakePage implements PageLifecycle {
  shown = true;
  readonly #listeners = new Map<string, Set<(event: { persisted?: boolean }) => void>>();

  visible(): boolean {
    return this.shown;
  }

  on(event: string, listener: (event: { persisted?: boolean }) => void): () => void {
    const set = this.#listeners.get(event) ?? new Set<(event: { persisted?: boolean }) => void>();
    this.#listeners.set(event, set);
    set.add(listener);
    return () => set.delete(listener);
  }

  /** fire calls the listeners of event; persisted tells a pageshow from the back-forward cache. */
  fire(event: string, persisted = false): void {
    for (const listener of this.#listeners.get(event) ?? []) {
      listener({ persisted });
    }
  }

  show(shown: boolean): void {
    this.shown = shown;
    this.fire("visibilitychange");
  }

  /** How many listeners the hub, and the edits, keep on the page. */
  listening(): number {
    return [...this.#listeners.values()].reduce((n, set) => n + set.size, 0);
  }
}
