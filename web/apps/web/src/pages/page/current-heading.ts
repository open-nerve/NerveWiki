import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

import { watchReader } from "./readers-input";

/**
 * useCurrentHeading is the section read of the page (M13/P4/S1, decisions 861, 887, 893): the id of the last of ids
 * (the outline's, in their order) whose heading's top is above the line, a third of the window down; the window's
 * bottom where the page scrolls at all and is at its end. A heading in a closed details (a folded callout) is not
 * read, its box what it may be. While active only (the right column open beside the page, decision 886) and some
 * heading is listed: not, it listens to nothing, measures nothing, and answers none. It measures as it mounts, in the commit (no frame later sets it: the tests'
 * act, section 1 (k)), then at most once a frame as the window scrolls or resizes and as the main resizes (the page
 * written in, its diagrams drawn), setting the state only as the answer changes. pin marks the heading of id
 * clicked in the outline (link, its link) at once: until the view has scrolled it in, and while it is in, until the
 * reader moves (watchReader), it is the current one.
 */
export function useCurrentHeading(
  ids: readonly string[],
  active: boolean
): { current: string | undefined; pin: (id: string, link: Element) => void } {
  const [current, setCurrent] = useState<string>();
  // What was set last: a frame that reads the same sets nothing, so that no update comes from a frame where nothing moved.
  const shown = useRef<string | undefined>(undefined);
  const show = useCallback((id: string | undefined) => {
    if (shown.current !== id) {
      shown.current = id;
      setCurrent(id);
    }
  }, []);
  const pinned = useRef<{ id: string; reader: ReturnType<typeof watchReader>; seen: boolean }>(undefined);
  const ask = useRef<() => void>(() => {});
  const live = useRef(false);
  useLayoutEffect(() => {
    // Folded away or not beside the page, or no heading listed (nothing to read, review A's P4S1A-N1): none and no pin,
    // so that shown again it answers what it measures (review A's R1-N5, R2-N2).
    const reading = active && ids.length > 0;
    live.current = reading;
    if (!reading) {
      pinned.current?.reader.end();
      pinned.current = undefined;
      show(undefined);
      return undefined;
    }
    let frame = 0;
    const read = (): string | undefined => {
      const { innerHeight, scrollY } = window;
      const { scrollHeight } = document.documentElement;
      // The end's rule where the page scrolls at all (decision 893): a page shorter than the window is never at its end.
      const atEnd = scrollHeight > innerHeight + 1 && scrollY + innerHeight >= scrollHeight - 1;
      const line = atEnd ? innerHeight : innerHeight / 3;
      const pin = pinned.current;
      if (pin !== undefined && !pin.reader.acted()) {
        const top = document.getElementById(pin.id)?.getBoundingClientRect().top;
        const inView = top !== undefined && top >= 0 && top < innerHeight;
        pin.seen ||= inView;
        // Until the view has scrolled it in (the router's commit may come a frame late: review A's R1-M1), and while in.
        if (inView || !pin.seen) {
          return pin.id;
        }
      }
      if (pin !== undefined) {
        pin.reader.end();
        pinned.current = undefined;
      }
      for (let i = ids.length - 1; i >= 0; i--) {
        const heading = document.getElementById(ids[i] ?? "");
        // A heading in a closed details (a folded callout) answers a box all the same (review A's I1): not read.
        if (heading === null || heading.closest("details:not([open])") !== null) {
          continue;
        }
        if (heading.getBoundingClientRect().top <= line) {
          return ids[i];
        }
      }
      return undefined;
    };
    const measured = () => {
      frame = 0;
      show(read());
    };
    const asked = () => {
      frame ||= requestAnimationFrame(measured);
    };
    ask.current = asked;
    // The first in the commit, before the paint: no frame later sets it (section 1 (k)).
    show(read());
    addEventListener("scroll", asked, { passive: true });
    addEventListener("resize", asked);
    const main = document.querySelector("main");
    const resized = typeof ResizeObserver === "undefined" || main === null ? undefined : new ResizeObserver(asked);
    if (main !== null) {
      resized?.observe(main);
    }
    return () => {
      cancelAnimationFrame(frame);
      removeEventListener("scroll", asked);
      removeEventListener("resize", asked);
      resized?.disconnect();
      ask.current = () => {};
    };
  }, [ids, active, show]);
  useEffect(() => () => pinned.current?.reader.end(), []);
  const pin = useCallback(
    (id: string, link: Element) => {
      // Not measured (folded away, or below the page): nothing marked, nothing watched (review A's R2-N2).
      if (!live.current) {
        return;
      }
      pinned.current?.reader.end();
      // The press on the link itself is no move of the reader's (watchReader's on); marked in the press's own handler.
      pinned.current = { id, reader: watchReader({ on: link }), seen: false };
      show(id);
      ask.current();
    },
    [show]
  );
  return { current: active && ids.length > 0 ? current : undefined, pin };
}
