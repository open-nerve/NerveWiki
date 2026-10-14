import { useEffect, useRef, useState, type RefObject } from "react";
import { unstable_serialize, type Cache, type ScopedMutator } from "swr";

import { useFocusLeaving } from "../lib/focus-leaving";
import { useMounted } from "../app/mounted";
import { Button } from "../components/ui/button";
import type { ListPage } from "../lib/read-pages";
import { watchReader } from "./page/readers-input";

/** What reading a list's next page answers: the ids it added, whether the list is read whole, and its last id. */
export type MoreRead = { added: readonly string[]; whole: boolean; last: string | undefined };

export type LoadMore = ReturnType<typeof useLoadMore>;

/**
 * readNext is the more of a list SWR keeps at key, read a page at a time
 * (readPages): it reads the page after cursor (read), then adds it to the
 * list cached at key, the key as More began, when that list still ends at
 * cursor, a read of it meanwhile having read it again; a read out as it is
 * added answers older than the addition, which SWR drops: the list is
 * read again whole. The list shown may have another key by then, a filter
 * chosen (M8/P3B review C-M1): its cursor names a place, not a list, so
 * the page goes to the list it was read for. idsOf are the ids of a page's
 * items.
 */
export async function readNext<T extends ListPage>(
  swr: { cache: Cache; mutate: ScopedMutator },
  key: readonly unknown[],
  cursor: string | undefined,
  read: (cursor: string | undefined) => Promise<T>,
  idsOf: (page: T) => readonly string[]
): Promise<MoreRead> {
  const next = await read(cursor);
  const list = await swr.mutate<T[]>(
    key,
    (cached) => (cached !== undefined && cached.at(-1)?.next_cursor === cursor ? [...cached, next] : cached),
    { revalidate: false }
  );
  if (swr.cache.get(unstable_serialize(key))?.isValidating === true) {
    // It rejects only once the login's cache is gone, the list with it (M8/P1 fix check 3, R3-1).
    void swr.mutate(key).catch(() => undefined);
  }
  const added = list?.at(-1) === next;
  const last = list?.at(-1);
  return {
    added: added ? idsOf(next) : [],
    whole: added && next.next_cursor === null,
    last: last === undefined ? undefined : idsOf(last).at(-1),
  };
}

/**
 * useLoadMore reads a list's next page (more). As the last is read, the
 * button goes: the focus falls to the first item it added, or the list's
 * last (itemOf finds it), or the list's title (heading), unless the reader
 * did something meanwhile or the focus is elsewhere than where the button
 * was (v0.1 design 13.2, item 26); the button gone with the focus gave it
 * to the title (left), which is where the button was. focusing is the item
 * the focus falls to, a list may hold its element by. A page whose list
 * changes under it, a filter chosen, names it (list): a read and a failure
 * are of the list they began on, and say nothing of another: a read that
 * ends as another is shown gives the focus nowhere, and a failure goes
 * once another list is shown (M8/P3B reviews C-M1, A-N1).
 */
export function useLoadMore(
  more: () => Promise<MoreRead>,
  itemOf: (id: string) => HTMLElement | null | undefined,
  heading: RefObject<HTMLElement | null>,
  list = ""
) {
  const mounted = useMounted();
  // The lists being read, and the failure of the last read, with its list.
  const [reading, setReading] = useState<readonly string[]>([]);
  const [failed, setFailed] = useState<{ list: string; error: unknown }>();
  // A failure is said while its list is shown: another shown, it goes.
  const [shown, setShown] = useState(list);
  if (shown !== list) {
    setShown(list);
    setFailed(undefined);
  }
  // Where the focus falls as the button goes: an item, or the title (null).
  const [focusing, setFocusing] = useState<string | null | undefined>(undefined);
  const busy = useRef(new Set<string>());
  // The list shown now, which a read begun on another does not move the focus in.
  const current = useRef(list);
  useEffect(() => {
    current.current = list;
  });
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
    const of = list;
    if (busy.current.has(of)) {
      return;
    }
    busy.current.add(of);
    setReading((lists) => [...lists, of]);
    setFailed(undefined);
    setFocusing(undefined);
    gaveTo.current = null;
    const reader = watchReader({ on: button.current });
    try {
      const { added, whole, last } = await more();
      const at = document.activeElement;
      const held =
        !reader.acted() && (at === button.current || at === null || at === document.body || at === gaveTo.current);
      if (mounted() && held && whole && current.current === of) {
        setFocusing(added[0] ?? last ?? null);
      }
    } catch (error) {
      if (mounted()) {
        setFailed({ list: of, error });
      }
    } finally {
      reader.end();
      busy.current.delete(of);
      if (mounted()) {
        setReading((lists) => lists.filter((each) => each !== of));
      }
    }
  }

  return {
    reading: reading.includes(list),
    failure: failed?.list === list ? failed.error : undefined,
    focusing,
    button,
    read,
    left,
  };
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
