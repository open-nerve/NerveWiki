import { expect, test } from "vitest";

import { oneAtATime, oneAtATimeById } from "./one-at-a-time";

/** A promise the test settles by hand. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

test("runs each task once the one before has settled, a failure included, and passes results on", async () => {
  const queue = oneAtATime();
  const first = deferred<string>();
  const started: string[] = [];

  const a = queue(() => {
    started.push("a");
    return first.promise;
  });
  const b = queue(async () => {
    started.push("b");
    return "b";
  });
  await Promise.resolve();
  expect(started).toEqual(["a"]);

  first.reject(new Error("a failed"));
  await expect(a).rejects.toThrow("a failed");
  await expect(b).resolves.toBe("b");
  expect(started).toEqual(["a", "b"]);
});

test("by id, a resource's tasks run one at a time, and another resource's beside them", async () => {
  const inTurn = oneAtATimeById();
  const first = deferred<void>();
  const started: string[] = [];

  const a1 = inTurn("a", async () => {
    started.push("a1");
    await first.promise;
  });
  const a2 = inTurn("a", async () => void started.push("a2"));
  const b1 = inTurn("b", async () => void started.push("b1"));
  await b1;
  expect(started).toEqual(["a1", "b1"]);

  first.resolve();
  await Promise.all([a1, a2]);
  expect(started).toEqual(["a1", "b1", "a2"]);
});
