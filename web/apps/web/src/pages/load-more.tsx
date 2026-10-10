import { useEffect, useRef, useState, type RefObject } from "react";

import { useFocusLeaving } from "../app/focus-leaving";
import { useMounted } from "../app/mounted";
import { Button } from "../components/ui/button";
import { watchReader } from "./page/readers-input";

/** What reading a list's next page answers: the ids it added, whether the list is read whole, and its last id. */
export type MoreRead = { added: readonly string[]; whole: boolean; last: string | undefined };

export type LoadMore = ReturnType<typeof useLoadMore>;

/**
 * useLoadMore reads a list's next page (more). As the last is read, the
 * button goes: the focus falls to the first item it added, or the list's
 * last (itemOf finds it), or the list's title (heading), unless the reader
 * did something meanwhile or the focus is elsewhere than where the button
 * was (v0.1 design 13.2, item 26); the button gone with the focus gave it
 * to the title (left), which is where the button was. focusing is the item
 * the focus falls to, a list may hold its element by.
 */
export function useLoadMore(
  more: () => Promise<MoreRead>,
  itemOf: (id: string) => HTMLElement | null | undefined,
  heading: RefObject<HTMLElement | null>
) {
  const mounted = useMounted();
  const [reading, setReading] = useState(false);
  const [failure, setFailure] = useState<unknown>(undefined);
  // Where the focus falls as the button goes: an item, or the title (null).
  const [focusing, setFocusing] = useState<string | null | undefined>(undefined);
  const busy = useRef(false);
  const button = useRef<HTMLButtonElement>(null);
  const item = useRef(itemOf);
  useEffect(() => {
    item.current = itemOf;
  });
  // The title, once the button gone with the focus gave it there.
  const gaveTo = useRef<Element | null>(null);
  useEffect(() => {
    if (focusing === undefined) {
      return;
    }
    const at = document.activeElement;
    if (at === null || at === document.body || at === gaveTo.current) {
      (focusing === null ? heading.current : item.current(focusing))?.focus({ preventScroll: true });
    }
  }, [focusing, heading]);

  function left(): void {
    gaveTo.current = heading.current;
    heading.current?.focus({ preventScroll: true });
  }

  async function read(): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setReading(true);
    setFailure(undefined);
    setFocusing(undefined);
    gaveTo.current = null;
    const reader = watchReader({ on: button.current });
    try {
      const { added, whole, last } = await more();
      const at = document.activeElement;
      const held =
        !reader.acted() && (at === button.current || at === null || at === document.body || at === gaveTo.current);
      if (mounted() && held && whole) {
        setFocusing(added[0] ?? last ?? null);
      }
    } catch (failed) {
      if (mounted()) {
        setFailure(failed);
      }
    } finally {
      reader.end();
      busy.current = false;
      if (mounted()) {
        setReading(false);
      }
    }
  }

  return { reading, failure, focusing, button, read, left };
}

/**
 * LoadMoreButton is the button that reads more, labelled label. Gone with
 * the focus, the last page read, it gives the focus to the list's title
 * before it leaves the document: not to the page's start.
 */
export function LoadMoreButton({ more, label }: { more: LoadMore; label: string }) {
  const { button, reading, read, left } = more;
  useFocusLeaving(button, left);
  return (
    <Button
      ref={button}
      variant="outline"
      aria-busy={reading || undefined}
      aria-disabled={reading || undefined}
      onClick={() => void read()}
    >
      {label}
    </Button>
  );
}
