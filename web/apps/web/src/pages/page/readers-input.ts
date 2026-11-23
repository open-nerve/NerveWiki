/**
 * readersInput are the events of what a reader does: a scroll, a click (the window's scrollbar pressed too, in
 * Chromium and WebKit), a touch, a key. An assistive technology's moves, a screen reader's virtual cursor, send none.
 */
export const readersInput = ["wheel", "touchmove", "pointerdown", "keydown"] as const;

/** The keys that press what has the focus again, or move nothing: Enter, Space, Escape, Shift, Control, Alt, Meta. */
const pressingAgain = new Set(["Enter", " ", "Shift", "Control", "Alt", "Meta", "Escape"]);

/**
 * watchReader watches what the reader does (readersInput) until it ends: acted tells whether they have done
 * something since it started, and acts is told the first time. On what on is, one of those keys (pressingAgain), or
 * a press of the main button, a touch's too (a button pressed again, a double click), is no move elsewhere: it is not
 * counted. Any other key (one that scrolls among them), another button's press, a scroll, a touch's move are. Given
 * within, a selector, only what is done in an element it matches counts: the left column's area, which the tree's
 * reveal leaves to a reader who moved in it (M13/P2/S2, decision 791).
 */
export function watchReader({ on, acts, within }: { on?: Element | null; acts?: () => void; within?: string } = {}): {
  acted: () => boolean;
  end: () => void;
} {
  let acted = false;
  const act = (event: Event) => {
    if (within !== undefined && !(event.target instanceof Element && event.target.closest(within) !== null)) {
      return;
    }
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
