import { observer } from "mobx-react-lite";
import { useRef, useState } from "react";
import { Link } from "react-router";
import useSWR from "swr";

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
import { PanelSection } from "./panel-section";

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
 * it; read again, as an event says the page's backlinks changed, the list
 * starts from its first page.
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
  // The pages of the list read, from its first.
  const { data, error, mutate } = useSWR(["backlinks", notebook.id, page], async () => [await pages.backlinks(page)]);
  const [reading, setReading] = useState(false);
  const busy = useRef(false);
  const [failure, setFailure] = useState<unknown>(undefined);

  async function more(after: BacklinkPage, cursor: string): Promise<void> {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setReading(true);
    setFailure(undefined);
    try {
      const next = await pages.backlinks(page, cursor);
      // Added after the page it was read after: a list read again meanwhile starts anew without it.
      await mutate((read) => (read?.at(-1) === after ? [...read, next] : read), { revalidate: false });
    } catch (failed) {
      if (mounted()) {
        setFailure(failed);
      }
    } finally {
      busy.current = false;
      if (mounted()) {
        setReading(false);
      }
    }
  }

  if (data === undefined) {
    return (
      <PanelSection title={t("page.backlinks")}>
        <NotLoaded error={error} retry={() => void mutate()} />
      </PanelSection>
    );
  }
  const shown: Shown[] = [];
  for (const { data: links } of data) {
    for (const { id, count, contexts } of links) {
      const node = pages.byId(id);
      if (node !== undefined) {
        shown.push({ id, name: node.name, count, contexts });
      }
    }
  }
  const last = data.at(-1);
  const cursor = last?.next_cursor ?? undefined;
  return (
    <PanelSection title={t("page.backlinks")}>
      {shown.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("page.noBacklinks")}</p>
      ) : (
        <ul className="space-y-3 text-sm">
          {shown.map(({ id, name, count, contexts }) => (
            <li key={id} className="space-y-1">
              <div className="break-words">
                <Link to={href(id)} state={arrived} className="underline underline-offset-4">
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
                    <li key={index} className="break-words whitespace-pre-wrap">
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
      {last !== undefined && cursor !== undefined && (
        <Button
          variant="outline"
          aria-busy={reading || undefined}
          aria-disabled={reading || undefined}
          onClick={() => void more(last, cursor)}
        >
          {t("page.more")}
        </Button>
      )}
    </PanelSection>
  );
});
