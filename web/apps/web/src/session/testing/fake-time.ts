// Waiting under vitest's fake timers, for the auth tests: time moves only when a test moves it, and every
// wait has a deadline in fake time, so a test that would hang fails instead.

import { vi } from "vitest";

/** Moves the fake time on in 10 ms steps until done() holds; fails after `deadline` ms of fake time. */
export async function until(done: () => boolean, what: string, deadline = 20_000): Promise<void> {
  if (done()) return;
  if (deadline <= 0) throw new Error(`timed out waiting for ${what}`);
  await vi.advanceTimersByTimeAsync(10);
  return until(done, what, deadline - 10);
}

/** Records what a promise resolves to or throws, so a test can look without awaiting it. */
export function track<T>(promise: Promise<T>) {
  const result: { settled: boolean; value?: T; error?: unknown } = { settled: false };
  promise.then(
    (value) => Object.assign(result, { settled: true, value }),
    (error: unknown) => Object.assign(result, { settled: true, error })
  );
  return result;
}

/** What a promise resolves to or throws, once it settles; fails, instead of hanging, when it never does. */
export async function settle<T>(promise: Promise<T>, what: string) {
  const result = track(promise);
  await until(() => result.settled, what);
  return result;
}
