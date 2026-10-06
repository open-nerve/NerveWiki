import { afterEach, beforeEach, expect, test, vi } from "vitest";

import { autosave, autosavePause } from "./autosave";
import { EditorClosed, type EditorContext } from "./registry";
import { fakeControls } from "./testing/fake-controls";

// Autosave (M5/P5 design 3.4): the content is saved once it has rested
// unchanged for 2 seconds.

// The timers only: the rejections left unhandled are told after an immediate.
beforeEach(() => void vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] }));
afterEach(() => void vi.useRealTimers());

const context: EditorContext = {
  workspace: "lab",
  notebook: "n1",
  page: "p1",
  role: "editor",
  linkTargets: () => Promise.resolve([]),
  tags: () => Promise.resolve([]),
};

/** autosave built over fake controls. */
function built() {
  const fake = fakeControls();
  autosave.extension(context, fake.controls);
  return fake;
}

test("the pause is 2 seconds (M5 design 4.8)", () => {
  expect(autosavePause).toBe(2_000);
});

test("a change is saved once the content has rested 2 seconds, once", () => {
  const { controls, change } = built();

  change();
  vi.advanceTimersByTime(autosavePause - 1);
  expect(controls.save).not.toHaveBeenCalled();
  vi.advanceTimersByTime(1);
  expect(controls.save).toHaveBeenCalledOnce();
  vi.advanceTimersByTime(autosavePause * 5);
  expect(controls.save).toHaveBeenCalledOnce();
});

test("changes in a row are saved once, 2 seconds after the last", () => {
  const { controls, change } = built();

  change();
  vi.advanceTimersByTime(autosavePause - 500);
  change();
  vi.advanceTimersByTime(autosavePause - 500);
  expect(controls.save).not.toHaveBeenCalled();
  vi.advanceTimersByTime(500);
  expect(controls.save).toHaveBeenCalledOnce();
});

test("nothing is saved before a change", () => {
  const { controls } = built();

  vi.advanceTimersByTime(autosavePause * 10);
  expect(controls.save).not.toHaveBeenCalled();
});

test("closed, a change resting is not saved", () => {
  const { controls, change, close } = built();

  change();
  close();
  vi.advanceTimersByTime(autosavePause);
  expect(controls.save).not.toHaveBeenCalled();
});

test("a save that fails, or whose editor closed, is let go, no rejection left unhandled; the next rest saves again", async () => {
  const rejections: unknown[] = [];
  const record = (reason: unknown) => void rejections.push(reason);
  process.on("unhandledRejection", record);
  try {
    // A spy would handle what its call answers: the saves are plain, each answer made as it is asked.
    const answers = [() => Promise.reject(new EditorClosed()), () => Promise.reject(new Error("offline"))];
    let saves = 0;
    const fake = fakeControls();
    autosave.extension(context, { ...fake.controls, save: () => (answers[saves++] ?? (() => Promise.resolve()))() });

    fake.change();
    await vi.advanceTimersByTimeAsync(autosavePause);
    fake.change();
    await vi.advanceTimersByTimeAsync(autosavePause);
    fake.change();
    await vi.advanceTimersByTimeAsync(autosavePause);
    await new Promise((resolve) => setImmediate(resolve));
    expect(saves).toBe(3);
    expect(rejections).toEqual([]);
  } finally {
    process.off("unhandledRejection", record);
  }
});
