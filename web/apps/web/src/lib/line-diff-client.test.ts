import { afterEach, describe, expect, test, vi } from "vitest";

import {
  compareInWorker,
  compareNow,
  maxDiffBytes,
  smallDiff,
  type DiffRequest,
  type DiffWorker,
} from "./line-diff-client";
import type { LineDiffResult } from "./line-diff";

/** A fake worker: it keeps what it was sent and whether it was ended, and answers when told. */
function fakeWorker() {
  const listeners: ((event: MessageEvent<LineDiffResult>) => void)[] = [];
  const worker = {
    sent: [] as DiffRequest[],
    terminated: false,
    addEventListener: (_: "message", listener: (event: MessageEvent<LineDiffResult>) => void) =>
      listeners.push(listener),
    postMessage: (request: DiffRequest) => worker.sent.push(request),
    terminate: () => {
      worker.terminated = true;
    },
    answer: (result: LineDiffResult) => {
      for (const listener of listeners) {
        listener(new MessageEvent("message", { data: result }));
      }
    },
  };
  return worker;
}

afterEach(() => {
  vi.useRealTimers();
});

describe("compareNow", () => {
  test("compares small texts at once, and leaves larger ones", () => {
    expect(compareNow("a\n", "b\n")).toMatchObject({ kind: "changes", added: 1, removed: 1 });
    const half = "x\n".repeat(smallDiff.lines / 2);
    expect(compareNow(half, half)).toBeUndefined();
    expect(compareNow("x".repeat(smallDiff.length), "y")).toBeUndefined();
  });
});

describe("compareInWorker", () => {
  test("answers the worker's result and ends it", async () => {
    const worker = fakeWorker();
    const answer = compareInWorker("a", "b", new AbortController().signal, () => worker as DiffWorker);
    expect(worker.sent).toEqual([{ a: "a", b: "b" }]);
    worker.answer({ kind: "changes", changes: [], added: 0, removed: 0 });
    expect(await answer).toEqual({ kind: "changes", changes: [], added: 0, removed: 0 });
    expect(worker.terminated).toBe(true);
  });

  test("ends a worker that does not answer in time: too large", async () => {
    vi.useFakeTimers();
    const worker = fakeWorker();
    const answer = compareInWorker("a", "b", new AbortController().signal, () => worker as DiffWorker, 2000);
    vi.advanceTimersByTime(1999);
    expect(worker.terminated).toBe(false);
    vi.advanceTimersByTime(1);
    expect(await answer).toEqual({ kind: "tooLarge", added: null, removed: null });
    expect(worker.terminated).toBe(true);
  });

  test("ends the worker when the comparison is dropped", async () => {
    const worker = fakeWorker();
    const controller = new AbortController();
    const answer = compareInWorker("a", "b", controller.signal, () => worker as DiffWorker);
    controller.abort(new Error("replaced"));
    await expect(answer).rejects.toThrow("replaced");
    expect(worker.terminated).toBe(true);
  });

  test("starts no worker for texts larger than the bound together: too large", async () => {
    const start = vi.fn(() => fakeWorker() as DiffWorker);
    const half = "é".repeat(maxDiffBytes / 4 + 1);
    expect(await compareInWorker(half, half, new AbortController().signal, start)).toEqual({
      kind: "tooLarge",
      added: null,
      removed: null,
    });
    expect(start).not.toHaveBeenCalled();
  });
});
