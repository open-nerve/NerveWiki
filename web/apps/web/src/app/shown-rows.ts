import { useEffect, useState } from "react";

/** How many rows a list renders at once as it mounts: more than a viewport and a few screens of scrolling hold. */
const first = 100;

/** How many more rows it renders after each paint, until all. */
const step = 400;

/**
 * useShownRows is how many of a list's total rows to render (M12/P4/S4
 * design 3.6): first at once, then step more after each paint, until all.
 * While not all are rendered, through is asked how many must be, and those
 * are rendered at once, and stay. Grown by more than step since all were
 * rendered, the rest come a step at a time again from those rendered. A
 * list mounted anew starts again from the first.
 *
 * The next rows come in a task after the frame that painted these: a
 * frame, then a timeout. A tab in the background paints no frame, and
 * goes on as it comes back.
 */
export function useShownRows(total: number, through?: () => number): number {
  const [state, setState] = useState(() => ({ shown: Math.min(total, first), all: total <= first }));
  let { shown, all } = state;
  if (all && total - shown > step) {
    // Grown by more than a step: from those rendered on.
    all = false;
  } else if (all) {
    shown = total;
  }
  if (!all) {
    // Read only while not all are rendered: once they are, what the list must render does not concern it.
    shown = Math.max(shown, Math.min(total, through?.() ?? 0));
    if (shown >= total) {
      shown = total;
      all = true;
    }
  }
  if (shown !== state.shown || all !== state.all) {
    // As it renders: those raised by through stay, though what it asks for drops back.
    setState({ shown, all });
  }
  useEffect(() => {
    if (state.all) {
      return undefined;
    }
    let timer: ReturnType<typeof setTimeout> | undefined;
    const frame = requestAnimationFrame(() => {
      timer = setTimeout(() => setState((now) => (now.all ? now : { shown: now.shown + step, all: false })));
    });
    return () => {
      cancelAnimationFrame(frame);
      clearTimeout(timer);
    };
  }, [state]);
  return shown;
}
