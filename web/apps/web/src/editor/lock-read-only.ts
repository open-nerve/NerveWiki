import type { ReadyExtension } from "./registry";

/**
 * lockReadOnly sets the editor read-only once the edit's session is lost
 * (M5 design 4.9): the text stays, to be copied, and cannot be changed,
 * since no save would take it. It adds nothing to the editor's state.
 */
export const lockReadOnly: ReadyExtension = {
  name: "lock-read-only",
  extension(_context, controls) {
    const follow = () => controls.setReadOnly(controls.session().lost);
    follow();
    controls.onSessionChange(follow);
    return [];
  },
};
