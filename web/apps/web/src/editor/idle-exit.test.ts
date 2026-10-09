import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { idleExit, idleLimit } from "./idle-exit";
import type { EditorContext } from "./registry";
import { fakeControls } from "./testing/fake-controls";

// Idle exit (M5/P5 design 3.5): an edit without input for 30 minutes is
// left.

beforeEach(() => void vi.useFakeTimers());
afterEach(() => void vi.useRealTimers());

const context: EditorContext = {
  workspace: "lab",
  notebook: "n1",
  page: "p1",
  role: "editor",
  linkTargets: () => Promise.resolve([]),
  tags: () => Promise.resolve([]),
  uploadAsset: () => Promise.reject(new Error("no uploads")),
};

/** idleExit built over fake controls. */
function built() {
  const fake = fakeControls();
  idleExit.extension(context, fake.controls);
  return fake;
}

test("the limit is 30 minutes (M5 design 4.7; the owner's decision)", () => {
  expect(idleLimit).toBe(30 * 60_000);
});

test("an editor left alone 30 minutes from its opening is left, as idle", async () => {
  const { controls } = built();

  await vi.advanceTimersByTimeAsync(idleLimit - 1);
  expect(controls.leave).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(1);
  expect(controls.leave.mock.calls).toEqual([["idle"]]);
});

test("each change starts the 30 minutes again", async () => {
  const { controls, change } = built();

  await vi.advanceTimersByTimeAsync(idleLimit - 1000);
  change();
  await vi.advanceTimersByTimeAsync(idleLimit - 1000);
  change();
  await vi.advanceTimersByTimeAsync(idleLimit - 1);
  expect(controls.leave).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(1);
  expect(controls.leave).toHaveBeenCalledOnce();
});

test("an edit that could not be left is tried again 30 minutes later, a leave that failed too; one left is not", async () => {
  const { controls, close } = built();
  controls.leave
    .mockResolvedValueOnce(undefined)
    .mockRejectedValueOnce(new Error("offline"))
    .mockImplementationOnce(() => {
      close();
      return Promise.resolve();
    });

  await vi.advanceTimersByTimeAsync(idleLimit);
  expect(controls.leave).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(idleLimit);
  expect(controls.leave).toHaveBeenCalledTimes(2);
  await vi.advanceTimersByTimeAsync(idleLimit);
  expect(controls.leave).toHaveBeenCalledTimes(3);
  await vi.advanceTimersByTimeAsync(idleLimit * 3);
  expect(controls.leave).toHaveBeenCalledTimes(3);
});

test("closed, it does not leave", async () => {
  const { controls, change, close } = built();

  change();
  close();
  await vi.advanceTimersByTimeAsync(idleLimit * 2);
  expect(controls.leave).not.toHaveBeenCalled();
});
