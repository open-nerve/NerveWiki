import type { ReadyExtension } from "./registry";

/** How long the content rests unchanged before it is saved (M5 design 4.8). */
export const autosavePause = 2_000;

/**
 * autosave saves the content once it has rested unchanged for
 * autosavePause (M5/P5 design 3.4), through the controls' save, which
 * waits for a composition to end: half a word is not saved, and the word
 * is saved once it is. Mod+S meanwhile leaves nothing to send. A failed
 * save says so on the edit's status: the next pause, Mod+S or leaving
 * tries again. It adds nothing to the editor's state.
 */
export const autosave: ReadyExtension = {
  name: "autosave",
  extension(_context, controls) {
    let resting: ReturnType<typeof setTimeout> | undefined;
    controls.onChange(() => {
      clearTimeout(resting);
      resting = setTimeout(() => void controls.save().catch(() => undefined), autosavePause);
    });
    controls.onClose(() => clearTimeout(resting));
    return [];
  },
};
