/**
 * UnloadWarning has the browser ask before the page is left, closed or
 * reloaded, while any of its holders asks it to (v0.1 design 13.2, item
 * 21): the uploads going, which leaving the page would stop.
 */
export class UnloadWarning {
  readonly #on = new Set<string>();
  readonly #warn = (event: Event) => event.preventDefault();

  constructor(private readonly target: Pick<EventTarget, "addEventListener" | "removeEventListener"> = window) {}

  /** set has holder ask for the warning, or no longer. */
  set(holder: string, on: boolean): void {
    const before = this.#on.size > 0;
    if (on) {
      this.#on.add(holder);
    } else {
      this.#on.delete(holder);
    }
    const after = this.#on.size > 0;
    if (after && !before) {
      this.target.addEventListener("beforeunload", this.#warn);
    } else if (!after && before) {
      this.target.removeEventListener("beforeunload", this.#warn);
    }
  }
}
