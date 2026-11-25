import { Clock } from "lucide-react";
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
import { isolate, Isolated } from "../../i18n/isolated";
import type { Notebook } from "../../services/notebook.service";
import type { PageVersion, PageVersionPage } from "../../services/page.service";
import { usePageTree, useStore } from "../../stores/context";
import type { PageTreeStore } from "../../stores/page-tree.store";
import { LoadMoreButton, readNext, useLoadMore } from "../load-more";
import { kindText } from "../notebook/changeset-text";
import { PanelSection } from "./panel-section";

/**
 * useVersions reads the page's versions, newer first, by the key the
 * events read them again by (events/handlers.ts): as many pages of them as
 * the cache has, from the first, fewer when the list ends (readPages), so
 * that a list read again (an event, a refocus, a connection, the page come
 * back to) is as long as it was; key is the key, which More adds to.
 */
export function useVersions(notebook: Notebook, page: string) {
  const pages = usePageTree(notebook);
  const key = ["page-versions", notebook.id, page];
  const { cache } = useSWRConfig();
  const cached = () => cache.get(unstable_serialize(key));
  const { data, error, mutate } = useSWR(key, () =>
    readPages(pages, page, (cached()?.data as PageVersionPage[] | undefined)?.length ?? 1)
  );
  return { data, error, mutate, key };
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
 * Each a line as the mockup's .ver (M13/P4/S1, decisions 863, 868): the
 * time, then who wrote it, one run of text wrapping as one, the name in a
 * bdi; the marks after it, in the same run, the current one amber
 * (M13/P5/S3, decision 943).
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
  const swr = useSWRConfig();
  const { data, error, mutate, key } = useVersions(notebook, page);
  const summary = useRef<HTMLElement>(null);
  // The version's link the focus falls to as More goes.
  const focused = useRef<HTMLAnchorElement>(null);
  const cursor = data?.at(-1)?.next_cursor ?? undefined;
  const more = useLoadMore(
    async () =>
      readNext(
        swr,
        key,
        cursor,
        (after) => pages.versions(page, after),
        (read) => read.data.map((version) => version.id)
      ),
    () => focused.current,
    summary
  );

  if (data === undefined) {
    return (
      <PanelSection title={t("version.list")} icon={Clock}>
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
    <PanelSection title={t("version.list")} icon={Clock} summaryRef={summary}>
      <p className="text-xs text-faint">{t("version.listHint")}</p>
      <p className="text-xs text-faint">{t("version.kept")}</p>
      {versions.length > 0 && (
        <ul aria-label={t("version.list")} className="space-y-1.5 text-[13px]">
          {versions.map((version) => {
            const when = formatRelative(version.updated_at, now, locale);
            const time = formatDateTime(version.updated_at, locale);
            const current = version.id === newest;
            return (
              <li key={version.id} className="flex items-baseline gap-2">
                {/* The time and who wrote it one run of text, wrapping as one; the marks after it, in the same run,
                    each after a space: the text has the row's whole width, no lone "-" on a line of its own
                    (M13/P5/S3, decision 943; review B's P5B-M5). */}
                <div className="min-w-0 flex-1 break-words">
                  <Link
                    ref={version.id === more.focusing ? focused : undefined}
                    to={href(version.revision)}
                    state={arrived}
                    aria-label={t("version.linkName", { when, revision: version.revision, time })}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <time dateTime={version.updated_at} title={time}>
                      {when}
                    </time>
                  </Link>
                  {/* The dot drawn before it, in no text: what the tests read of the writer stays "Ada · Edit". */}
                  <p className="inline text-muted-foreground before:mx-1 before:content-['·']">
                    <VersionBy version={version} t={t} />
                  </p>
                  {current && (
                    <>
                      {" "}
                      <Mark current>{t("version.current")}</Mark>
                    </>
                  )}
                  {version.editing && (
                    <>
                      {" "}
                      <Mark>{t("version.editing")}</Mark>
                    </>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {more.failure !== undefined && <Alert>{errorText(more.failure, t)}</Alert>}
      {cursor !== undefined && <LoadMoreButton more={more} label={t("version.more")} small />}
    </PanelSection>
  );
});

/**
 * The plain mark's classes: round, as a badge is (settings-page.tsx's badgeClass; M13/P5/S3, decision 930), its words
 * one piece: in the right column's run of text a word of Chinese would break between its characters, half a mark on a
 * line (review B's P5B-R1-M2). The history's Undone link is one (cn(markClass, its hover)).
 */
export const markClass =
  "rounded-full border border-border bg-surface-2 px-1.5 text-[11px] whitespace-nowrap text-muted-foreground";

/**
 * Mark is a version's mark beside its time: current, or editing; current, the right column's current one, amber on
 * its soft ground, round (the mockup's .ver .tag; M13/P4/S1, decision 863), its border transparent but in forced
 * colours, which take its ground away (review B's P4S1B-N2); the version's page's current one too (decision 930): the
 * history has the plain one.
 */
export function Mark({ children, current = false }: { children: string; current?: boolean }) {
  return (
    <span
      className={
        current
          ? "rounded-full border border-transparent bg-accent-soft px-1.5 text-[11px] whitespace-nowrap text-accent"
          : markClass
      }
    >
      {children}
    </span>
  );
}

/**
 * versionBy says who wrote version, and what its changeset was: "Ada · Edit", a no-break space after the dot, so that
 * the version's page breaks its line before "· Edit", not between them (review B's P4S1B-F2-1). The version's page's.
 */
export function versionBy(version: PageVersion, t: Translate): string {
  return `${version.changeset.created_by.display_name} · ${kindText(version.changeset.kind, t)}`;
}

/** VersionBy is versionBy written with the name in a bdi (M13/P4/S1, decision 868): the right column's. */
function VersionBy({ version, t }: { version: PageVersion; t: Translate }) {
  return (
    <Isolated
      {...isolate(
        (name) => `${name(version.changeset.created_by.display_name)} · ${kindText(version.changeset.kind, t)}`
      )}
    />
  );
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
