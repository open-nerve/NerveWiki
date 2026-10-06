/**
 * readersInput are the events of what a reader does: a scroll, a click (the window's scrollbar pressed too, in
 * Chromium and WebKit), a touch, a key. An assistive technology's moves, a screen reader's virtual cursor, send none.
 */
export const readersInput = ["wheel", "touchmove", "pointerdown", "keydown"] as const;

/**
 * watchReader watches what the reader does (readersInput) until it ends: acted tells whether they have done
 * something since it started, and acts is told the first time. A key or a press on what on is (a button pressed
 * again) is not counted: it is no move elsewhere.
 */
export function watchReader({ on, acts }: { on?: Element | null; acts?: () => void } = {}): {
  acted: () => boolean;
  end: () => void;
} {
  let acted = false;
  const act = (event: Event) => {
    const pressed = event.type === "keydown" || event.type === "pointerdown";
    if (acted || (pressed && on && event.target instanceof Node && on.contains(event.target))) {
      return;
    }
    acted = true;
    acts?.();
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
