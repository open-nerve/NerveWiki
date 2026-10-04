import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { INTERVAL_MS, Refresher } from "./refresher";

/** A page whose visibility the test sets. */
function page(visible = true) {
  const listeners = new Set<() => void>();
  return {
    visible: () => visible,
    onChange: (listener: () => void) => {
      listeners.add(listener);
      return () => void listeners.delete(listener);
    },
    show: (now: boolean) => {
      visible = now;
      for (const listener of listeners) listener();
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers({ now: 1_000_000 });
});

afterEach(() => {
  vi.useRealTimers();
});

test("a visible tab reads a key at most once in 5 seconds (M5 design 4.11)", () => {
  expect(INTERVAL_MS).toBe(5_000);
});

describe("Refresher", () => {
  test("reads at once, then at most once an interval per key, the interval's last request", async () => {
    const reads: string[] = [];
    const refresher = new Refresher(page(), () => Date.now());

    refresher.request("a", () => reads.push("a1"));
    refresher.request("a", () => reads.push("a2"));
    refresher.request("a", () => reads.push("a3"));
    refresher.request("b", () => reads.push("b1"));
    expect(reads).toEqual(["a1", "b1"]);

    await vi.advanceTimersByTimeAsync(INTERVAL_MS - 1);
    expect(reads).toEqual(["a1", "b1"]);
    await vi.advanceTimersByTimeAsync(1);
    expect(reads).toEqual(["a1", "b1", "a3"]);
    await vi.advanceTimersByTimeAsync(INTERVAL_MS * 2);
    expect(reads).toEqual(["a1", "b1", "a3"]);
    refresher.request("a", () => reads.push("a4"));
    expect(reads).toEqual(["a1", "b1", "a3", "a4"]);
  });

  test("a hidden page reads once it is visible again, at most once an interval", async () => {
    const reads: string[] = [];
    const p = page(false);
    const refresher = new Refresher(p, () => Date.now());

    refresher.request("a", () => reads.push("a1"));
    refresher.request("a", () => reads.push("a2"));
    await vi.advanceTimersByTimeAsync(INTERVAL_MS * 3);
    expect(reads).toEqual([]);

    p.show(true);
    expect(reads).toEqual(["a2"]);
    refresher.request("a", () => reads.push("a3"));
    p.show(false);
    await vi.advanceTimersByTimeAsync(INTERVAL_MS);
    expect(reads).toEqual(["a2"]);
    p.show(true);
    expect(reads).toEqual(["a2", "a3"]);
  });

  test("stop drops what is due", async () => {
    const reads: string[] = [];
    const refresher = new Refresher(page(), () => Date.now());
    refresher.request("a", () => reads.push("a1"));
    refresher.request("a", () => reads.push("a2"));

    refresher.stop();
    await vi.advanceTimersByTimeAsync(INTERVAL_MS);

    expect(reads).toEqual(["a1"]);
    expect(vi.getTimerCount()).toBe(0);
  });
});
