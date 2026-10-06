import { observer } from "mobx-react-lite";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { useMounted } from "../../app/mounted";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { Button } from "../../components/ui/button";
import { useT } from "../../i18n/i18n";
import type { BacklinkPage } from "../../services/linking.service";
import type { Notebook } from "../../services/notebook.service";
import { usePageTree } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { PanelSection } from "./panel-section";
import { watchReader } from "./readers-input";

/** How many links of a page the server counts: as many as this is as many or more. */
const countedUpTo = 1000;

/** A page that links here, as the list shows it. */
type Shown = { id: string; name: string; count: number; contexts: string[] };

/**
 * PageBacklinks is the pages that link to the page (M6/P7 design 9), as the
 * link index has them, a page of them at a time: each by its title in the
 * notebook's tree, leading there, with how many of its links lead here
 * when more than one, and the lines of its first ones, as the server
 * writes them. One the tree does not have yet, made in another tab, shows
 * once the tree is read again. More reads the next page of them and adds
 * it; the last, More going, the focus falls to the first page it adds
 * that shows (or the last that shows, or the section's title), without a
 * scroll, unless the reader has done something meanwhile (scrolled,
 * clicked, touched, pressed a key) or put the focus elsewhere than More
 * (which a click in Safari, or a screen reader, may give no focus). Read
 * again (an event, a refocus, a connection, the page come back to), the
 * list is as many pages as were read, from the first.
 */
export const PageBacklinks = observer(function PageBacklinks({
  notebook,
  page,
  href,
}: {
  notebook: Notebook;
  page: string;
  href: (id: string) => string;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const mounted = useMounted();
  const key = ["backlinks", notebook.id, page];
  const { cache } = useSWRConfig();
  // A read reads as many pages as the list has, by the cache as it reads: what More added, the page come back to.
  const cached = () => cache.get(unstable_serialize(key));
  const { data, error, mutate } = useSWR(key, () =>
    readPages(pages, page, (cached()?.data as BacklinkPage[] | undefined)?.length ?? 1)
  );
  const [reading, setReading] = useState(false);
  const busy = useRef(false);
  const [failure, setFailure] = useState<unknown>(undefined);
  // Where the focus falls as More goes: the link of a page that links here, or the section's title (null).
  const [focusing, setFocusing] = useState<string | null | undefined>(undefined);
  const focused = useRef<HTMLAnchorElement>(null);
  const summary = useRef<HTMLElement>(null);
  const moreButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (focusing === undefined) {
      return;
    }
    setFocusing(undefined);
    // Only from where More's going left it (a race: the focus moved as the list was added to keeps it there). Where
    // More was: no scroll.
    const at = document.activeElement;
    if (at === null || at === document.body) {
      (focusing === null ? summary.current : focused.current)?.focus({ preventScroll: true });
    }
  }, [focusing]);

  async function more(cursor: string): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setReading(true);
    setFailure(undefined);
    const reader = watchReader();
    try {
      const next = await pages.backlinks(page, cursor);
      // Whether the focus is More's still as it answers: on More or nowhere, the reader having done nothing since.
      const at = document.activeElement;
      const held = !reader.acted() && (at === moreButton.current || at === null || at === document.body);
      // Added after the page it was read after, which a read meanwhile may have read again.
      const read = await mutate((list) => (list?.at(-1)?.next_cursor === cursor ? [...list, next] : list), {
        revalidate: false,
      });
      if (cached()?.isValidating === true) {
        // A read out as it was added (this view's or the page's come back to) answers what is older than the
        // addition: SWR drops it. Another reads it all.
        void mutate();
      }
      if (mounted() && held && read?.at(-1) === next && next.next_cursor === null) {
        // More goes: the focus to the first page it added that shows, or the last that shows, or the title.
        const added = next.data.find(({ id }) => pages.byId(id) !== undefined);
        setFocusing(added?.id ?? shownOf(pages, read).at(-1)?.id ?? null);
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

  if (data === undefined) {
    return (
      <PanelSection title={t("page.backlinks")}>
        <div className="text-sm">
          <NotLoaded error={error} retry={() => void mutate()} />
        </div>
      </PanelSection>
    );
  }
  const shown = shownOf(pages, data);
  const cursor = data.at(-1)?.next_cursor ?? undefined;
  return (
    <PanelSection title={t("page.backlinks")} summaryRef={summary}>
      {shown.length === 0 && cursor === undefined && (
        <p className="text-sm text-muted-foreground">{t("page.noBacklinks")}</p>
      )}
      {shown.length > 0 && (
        <ul className="space-y-3 text-sm">
          {shown.map(({ id, name, count, contexts }) => (
            <li key={id} className="space-y-1">
              <div className="break-words">
                <Link
                  ref={id === focusing ? focused : undefined}
                  to={href(id)}
                  state={arrived}
                  className="underline underline-offset-4"
                >
                  {name}
                </Link>
                {count > 1 && (
                  <span className="text-muted-foreground">
                    {" · "}
                    {t("page.backlinkCount", { count: count >= countedUpTo ? `${countedUpTo}+` : String(count) })}
                  </span>
                )}
              </div>
              {contexts.length > 0 && (
                <ul className="space-y-1 text-xs text-muted-foreground">
                  {contexts.map((line, index) => (
                    // oxlint-disable-next-line react/no-array-index-key -- two lines may read the same: by where they are
                    <li key={index} className="break-words">
                      {line}
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ))}
        </ul>
      )}
      {failure !== undefined && <Alert>{errorText(failure, t)}</Alert>}
      {cursor !== undefined && (
        <Button
          ref={moreButton}
          variant="outline"
          aria-busy={reading || undefined}
          aria-disabled={reading || undefined}
          onClick={() => void more(cursor)}
        >
          {t("page.moreBacklinks")}
        </Button>
      )}
    </PanelSection>
  );
});

/** readPages reads count pages of the pages that link to the page id, from the first, fewer when the list ends. */
async function readPages(pages: PageTreeStore, id: string, count: number): Promise<BacklinkPage[]> {
  const read = [await pages.backlinks(id)];
  for (let cursor = read[0]?.next_cursor; cursor && read.length < count; cursor = read.at(-1)?.next_cursor) {
    // oxlint-disable-next-line no-await-in-loop -- each page after the one before
    read.push(await pages.backlinks(id, cursor));
  }
  return read;
}

/** shownOf is what the list shows of the pages read: those that link here the tree has, by their title. */
function shownOf(pages: PageTreeStore, read: readonly BacklinkPage[]): Shown[] {
  const shown: Shown[] = [];
  for (const { data: links } of read) {
    for (const { id, count, contexts } of links) {
      const node = pages.byId(id);
      if (node !== undefined) {
        shown.push({ id, name: node.name, count, contexts });
      }
    }
  }
  return shown;
}
