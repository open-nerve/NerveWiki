/**
 * readersInput are the events of what a reader does: a scroll, a click (the window's scrollbar pressed too, in
 * Chromium and WebKit), a touch, a key. An assistive technology's moves, a screen reader's virtual cursor, send none.
 */
export const readersInput = ["wheel", "touchmove", "pointerdown", "keydown"] as const;

/** The keys that press what has the focus again, or move nothing: Enter, Space, a modifier, Escape. */
const pressingAgain = new Set(["Enter", " ", "Shift", "Control", "Alt", "Meta", "Escape"]);

/**
 * watchReader watches what the reader does (readersInput) until it ends: acted tells whether they have done
 * something since it started, and acts is told the first time. On what on is, a key that presses it again or moves
 * nothing, or a press of the main button (a button pressed again, a double click), is no move elsewhere: it is not
 * counted. A key that scrolls, a scroll, a touch's move are.
 */
export function watchReader({ on, acts }: { on?: Element | null; acts?: () => void } = {}): {
  acted: () => boolean;
  end: () => void;
} {
  let acted = false;
  const act = (event: Event) => {
    const again =
      (event instanceof KeyboardEvent && pressingAgain.has(event.key)) ||
      (event.type === "pointerdown" && event instanceof MouseEvent && event.button === 0);
    if (acted || (again && on && event.target instanceof Node && on.contains(event.target))) {
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
