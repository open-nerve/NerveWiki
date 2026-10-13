import { afterEach, describe, expect, test, vi } from "vitest";

import { compareInWorker, compareNow, smallDiff, type DiffRequest, type DiffWorker } from "./line-diff-client";
import type { LineDiffResult } from "./line-diff";

/** A fake worker: it keeps what it was sent and whether it was ended, and answers or fails when told. */
function fakeWorker() {
  const listeners: { type: string; listener: (event: MessageEvent<LineDiffResult>) => void }[] = [];
  const fire = (type: string, event: MessageEvent<LineDiffResult>) => {
    for (const each of listeners.filter((listening) => listening.type === type)) {
      each.listener(event);
    }
  };
  const worker = {
    sent: [] as DiffRequest[],
    terminated: false,
    addEventListener: (type: string, listener: (event: MessageEvent<LineDiffResult>) => void) =>
      listeners.push({ type, listener }),
    postMessage: (request: DiffRequest) => worker.sent.push(request),
    terminate: () => {
      worker.terminated = true;
    },
    answer: (result: LineDiffResult) => fire("message", new MessageEvent("message", { data: result })),
    fail: (type: "error" | "messageerror") => fire(type, new MessageEvent(type)),
  };
  return worker;
}

/** noWorker is a browser's refusal to start a worker. */
function noWorker(): DiffWorker {
  throw new Error("no workers here");
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

  test("ends a worker that does not answer in two seconds: too large", async () => {
    vi.useFakeTimers();
    const worker = fakeWorker();
    const answer = compareInWorker("a", "b", new AbortController().signal, () => worker as DiffWorker);
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

  test("ends a worker whose script does not load, or whose answer cannot be read: the comparison fails", async () => {
    const failing = ["error", "messageerror"] as const;
    const workers = failing.map(() => fakeWorker());
    const answers = workers.map((worker) =>
      compareInWorker("a", "b", new AbortController().signal, () => worker as DiffWorker)
    );
    for (const [i, worker] of workers.entries()) {
      worker.fail(failing[i] ?? "error");
    }
    await Promise.all(answers.map((answer) => expect(answer).rejects.toThrow("the comparison's worker failed")));
    expect(workers.map((worker) => worker.terminated)).toEqual([true, true]);
  });

  test("fails when no worker can start", async () => {
    await expect(compareInWorker("a", "b", new AbortController().signal, noWorker)).rejects.toThrow("no workers here");
  });

  test("starts no worker for texts of more than 5 MiB of UTF-8 together: too large", async () => {
    const start = vi.fn(() => fakeWorker() as DiffWorker);
    // Two bytes each.
    const half = "é".repeat((5 * 1024 * 1024) / 4);
    const controller = new AbortController();
    const atTheBound = compareInWorker(half, half, controller.signal, start);
    expect(start).toHaveBeenCalledOnce();
    controller.abort(new Error("compared"));
    await expect(atTheBound).rejects.toThrow("compared");
    expect(await compareInWorker(half, `${half}x`, new AbortController().signal, start)).toEqual({
      kind: "tooLarge",
      added: null,
      removed: null,
    });
    expect(start).toHaveBeenCalledOnce();
  });
});
