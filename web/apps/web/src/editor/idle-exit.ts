import type { EditorExtension } from "./registry";

/** How long an edit may go without input before it is left (M5 design 4.7). */
export const idleLimit = 30 * 60_000;

/**
 * idleExit leaves the edit once its content has gone unchanged for
 * idleLimit, from when the editor opened or the last change (M5/P5 design
 * 3.5): saved, its session ended, the reading view saying why. An edit
 * that could not be left is tried again after as long. A hidden tab's
 * timer runs late by up to a minute, a frozen tab's once it thaws. It adds
 * nothing to the editor's state.
 */
export const idleExit: EditorExtension = {
  name: "idle-exit",
  extension(_context, controls) {
    let idle: ReturnType<typeof setTimeout> | undefined;
    let closed = false;
    function wait() {
      clearTimeout(idle);
      idle = setTimeout(() => void leave(), idleLimit);
    }
    async function leave() {
      await controls.leave("idle").catch(() => undefined);
      if (!closed) {
        wait();
      }
    }
    wait();
    controls.onChange(wait);
    controls.onClose(() => {
      closed = true;
      clearTimeout(idle);
    });
    return [];
  },
};
