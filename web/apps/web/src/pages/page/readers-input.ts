/**
 * readersInput are the events of what a reader does: a scroll, a click (the window's scrollbar pressed too, in
 * Chromium and WebKit), a touch, a key. An assistive technology's moves, a screen reader's virtual cursor, send none.
 */
export const readersInput = ["wheel", "touchmove", "pointerdown", "keydown"] as const;

/** watchReader tells, until it ends, whether the reader has done something (readersInput) since it started. */
export function watchReader(): { acted: () => boolean; end: () => void } {
  let acted = false;
  const act = () => {
    acted = true;
  };
  for (const type of readersInput) {
    window.addEventListener(type, act, { capture: true, passive: true });
  }
  return {
    acted: () => acted,
    end: () => {
      for (const type of readersInput) {
        window.removeEventListener(type, act, true);
      }
    },
  };
}
