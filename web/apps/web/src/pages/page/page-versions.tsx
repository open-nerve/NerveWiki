import { observer } from "mobx-react-lite";
import { useRef } from "react";
import { Link } from "react-router";
import useSWR, { unstable_serialize, useSWRConfig } from "swr";

import { arrived } from "../../app/arrival";
import { NotLoaded } from "../../app/not-loaded";
import { errorText } from "../../app/problem-messages";
import { Alert } from "../../components/ui/alert";
import { formatDateTime, formatRelative } from "../../i18n/format";
import { useT, type Translate } from "../../i18n/i18n";
import type { Notebook } from "../../services/notebook.service";
import type { PageVersion, PageVersionPage } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { LoadMoreButton, useLoadMore } from "../load-more";
import { PanelSection } from "./panel-section";

/**
 * useVersions reads the page's versions, newer first, by the key the
 * events read them again by (events/handlers.ts): as many pages of them as
 * the cache has, from the first, fewer when the list ends (readPages), so
 * that a list read again (an event, a refocus, a connection, the page come
 * back to) is as long as it was. cached is the key's state in SWR's cache.
 */
export function useVersions(notebook: Notebook, page: string) {
  const pages = usePageTree(notebook);
  const key = ["page-versions", notebook.id, page];
  const { cache } = useSWRConfig();
  const cached = () => cache.get(unstable_serialize(key));
  const { data, error, mutate } = useSWR(key, () =>
    readPages(pages, page, (cached()?.data as PageVersionPage[] | undefined)?.length ?? 1)
  );
  return { data, error, mutate, cached };
}

/**
 * PageVersions is the page's versions in its right column (M8 design 4.3,
 * 4.13), one for each edit, newer first, a page of them at a time: each
 * when it was written, as long ago as it is (the time in full as its
 * title, and in its name with the version's revision, which tell apart two
 * of one hour), leading to its version's page, then who wrote it and what
 * its changeset was; the newest is marked current, and editing while its
 * session is alive. More reads the next page and adds it; the last, More
 * going, the focus falls to the first version it adds (load-more.tsx).
 */
export const PageVersions = observer(function PageVersions({
  notebook,
  page,
  href,
}: {
  notebook: Notebook;
  page: string;
  /** The address of the page's version of revision. */
  href: (revision: number) => string;
}) {
  const t = useT();
  const pages = usePageTree(notebook);
  const { preferences } = useStore();
  const { data, error, mutate, cached } = useVersions(notebook, page);
  const summary = useRef<HTMLElement>(null);
  // The version's link the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const more = useLoadMore(
    async () => {
      const next = await pages.versions(page, cursor);
      // Added after the page it was read after, which a read meanwhile may have read again.
      const read = await mutate(
        (list) => (list !== undefined && list.at(-1)?.next_cursor === cursor ? [...list, next] : list),
        { revalidate: false }
      );
      if (cached()?.isValidating === true) {
        // A read out as it was added answers what is older than the addition: SWR drops it. Another reads it all.
        void mutate();
      }
      const added = read?.at(-1) === next;
      return {
        added: added ? next.data.map((version) => version.id) : [],
        whole: added && next.next_cursor === null,
        last: read?.at(-1)?.data.at(-1)?.id,
      };
    },
    () => focused.current,
    summary
  );

  if (data === undefined) {
    return (
      <PanelSection title={t("version.list")}>
        <div className="text-sm">
          <NotLoaded error={error} retry={() => void mutate()} />
        </div>
      </PanelSection>
    );
  }
  const versions = data.flatMap((read) => read.data);
  const newest = data[0]?.data[0]?.id;
  const { locale } = preferences;
  const now = Date.now();
  return (
    <PanelSection title={t("version.list")} summaryRef={summary}>
      <p className="text-xs text-muted-foreground">{t("version.listHint")}</p>
      {versions.length > 0 && (
        <ul aria-label={t("version.list")} className="space-y-2 text-sm">
          {versions.map((version) => {
            const when = formatRelative(version.updated_at, now, locale);
            const time = formatDateTime(version.updated_at, locale);
            return (
              <li key={version.id} className="space-y-0.5">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <Link
                    ref={version.id === more.focusing ? focused : undefined}
                    to={href(version.revision)}
                    state={arrived}
                    aria-label={t("version.linkName", { when, revision: version.revision, time })}
                    className="underline underline-offset-4"
                  >
                    <time dateTime={version.updated_at} title={time}>
                      {when}
                    </time>
                  </Link>
                  {version.id === newest && <Mark>{t("version.current")}</Mark>}
                  {version.editing && <Mark>{t("version.editing")}</Mark>}
                </div>
                <p className="text-xs break-words text-muted-foreground">{versionBy(version, t)}</p>
              </li>
            );
          })}
        </ul>
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {cursor !== undefined && <LoadMoreButton more={more} label={t("version.more")} />}
    </PanelSection>
  );
});

/** Mark is a version's mark beside its time: current, or editing. */
export function Mark({ children }: { children: string }) {
  return <span className="rounded border px-1.5 text-xs text-muted-foreground">{children}</span>;
}

/** versionBy says who wrote version, and what its changeset was: "Ada · Edit". */
export function versionBy(version: PageVersion, t: Translate): string {
  return `${version.changeset.created_by.display_name} · ${kindText(version.changeset.kind, t)}`;
}

/** kindText is the name of a changeset's kind, every kind the contract has. */
function kindText(kind: PageVersion["changeset"]["kind"], t: Translate): string {
  switch (kind) {
    case "edit":
      return t("version.kind.edit");
    case "import":
      return t("version.kind.import");
    case "version_restore":
      return t("version.kind.version_restore");
    case "trash_restore":
      return t("version.kind.trash_restore");
    case "revert":
      return t("version.kind.revert");
  }
}

/** readPages reads count pages of the page id's versions, from the first, fewer when the list ends. */
async function readPages(pages: PageTreeStore, id: string, count: number): Promise<PageVersionPage[]> {
  const read = [await pages.versions(id)];
  for (let cursor = read[0]?.next_cursor; cursor && read.length < count; cursor = read.at(-1)?.next_cursor) {
    // oxlint-disable-next-line no-await-in-loop -- each page after the one before
    read.push(await pages.versions(id, cursor));
  }
  return read;
}
