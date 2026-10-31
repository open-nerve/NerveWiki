import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";

import type { LoadMore, MoreRead } from "../load-more";
import { watchReader } from "../page/readers-input";

/** Health moves to actual additions on every page; the shared search policy stays unchanged. */
export function useHealthLoadMore(
  readPage: () => Promise<MoreRead>,
  current: () => boolean,
  itemOf: (id: string) => HTMLElement | null | undefined,
  heading: RefObject<HTMLElement | null>,
  identity: string,
  disabled: boolean,
  validating: boolean
): LoadMore {
  const [reading, setReading] = useState<string>();
  const [focus, setFocus] = useState<{ identity: string; id: string | null }>();
  const button = useRef<HTMLButtonElement>(null);
  const busy = useRef(new Set<string>());
  const readers = useRef(new Map<ReturnType<typeof watchReader>, string>());
  const blocked = useRef<string | undefined>(undefined);
  const gaveTo = useRef<Element | null>(null);
  const latest = useRef({ current, itemOf, identity });
  useLayoutEffect(() => {
    latest.current = { current, itemOf, identity };
  });
  useEffect(
    () => () => {
      for (const [reader, of] of readers.current) {
        if (of === identity) {
          reader.end();
          readers.current.delete(reader);
        }
      }
    },
    [identity]
  );
  useLayoutEffect(() => {
    const view = latest.current;
    if (focus === undefined || focus.identity !== view.identity || !view.current()) return;
    const at = document.activeElement;
    if (at === button.current || at === document.body || at === null || at === gaveTo.current) {
      (focus.id === null ? heading.current : view.itemOf(focus.id))?.focus({ preventScroll: true });
    }
  }, [focus, heading]);

  function left() {
    if (
      !latest.current.current() ||
      !busy.current.has(latest.current.identity) ||
      blocked.current === latest.current.identity
    )
      return;
    gaveTo.current = heading.current;
    heading.current?.focus({ preventScroll: true });
  }

  async function read() {
    if (disabled || validating || !current() || busy.current.has(identity)) return;
    busy.current.add(identity);
    setReading(identity);
    setFocus(undefined);
    gaveTo.current = null;
    blocked.current = undefined;
    const reader = watchReader({
      on: button.current,
      acts: () => {
        blocked.current = identity;
      },
    });
    readers.current.set(reader, identity);
    try {
      const result = await readPage();
      const at = document.activeElement;
      if (
        current() &&
        latest.current.identity === identity &&
        !reader.acted() &&
        (at === button.current || at === document.body || at === null || at === gaveTo.current) &&
        (result.added.length > 0 || result.whole)
      ) {
        setFocus({ identity, id: result.added[0] ?? result.last ?? null });
      }
    } catch {
      // The card's SWR state renders its error, including when older results remain.
    } finally {
      reader.end();
      readers.current.delete(reader);
      busy.current.delete(identity);
      if (current()) setReading((was) => (was === identity ? undefined : was));
    }
  }

  return {
    reading: reading === identity || validating,
    failure: undefined,
    focusing: focus?.identity === identity ? focus.id : undefined,
    button,
    read,
    left,
  };
}
