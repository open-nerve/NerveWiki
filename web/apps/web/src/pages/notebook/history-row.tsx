import { observer } from "mobx-react-lite";
import { useRef, type Ref } from "react";
import { Link } from "react-router";

import { arrived } from "../../app/arrival";
import { factsClass, lineLinkClass, lineTitleClass, revealClass, stackedLinesClass } from "../../app/page-classes";
import { Button } from "../../components/ui/button";
import { formatDateTime, formatRelative } from "../../i18n/format";
import { useT } from "../../i18n/i18n";
import { isolate, Isolated } from "../../i18n/isolated";
import { cn } from "../../lib/cn";
import { useFocusLeaving } from "../../lib/focus-leaving";
import { shortId } from "../../lib/twins";
import type { Changeset } from "../../services/page.service";
import { useStore } from "../../stores/context";
import { Mark, markClass } from "../page/page-versions";
import { byText, changesetName, changesText, clientText, countsText } from "./changeset-text";
import { IngestMark, IngestSources } from "./history-ingest";

/**
 * HistoryRow is a changeset of a notebook's history (M8 design 4.7, 4.13;
 * M8/P3 design 4.2): its kind and who made it, which leads to its page and
 * is named by when it was made, to the minute, a twin by the end of its id
 * too, which the row shows (twin); Only this person beside them, unless
 * the list is theirs already (only), named by who, an account of the name
 * of another told apart as the filter tells it; where it came from, as long ago
 * before now as it is (the time in full as its title), why (its message),
 * its first changes summed up and its counts; and its marks: undone,
 * leading to its newest revert, trimmed, and what a revert undid, leading
 * there. Gone with the focus, read away, it gives the focus away (left)
 * before it leaves. Its links arrive where they lead (M13/P5/S2, decision
 * 940); Only this person is what it offers besides them, shown as it is
 * hovered or focused (revealClass, decision 927).
 */
export const HistoryRow = observer(function HistoryRow({
  changeset,
  href,
  twin,
  revertsTwin,
  only,
  left,
  linkRef,
  now,
}: {
  changeset: Changeset;
  /** The address of the page of the changeset id. */
  href: (id: string) => string;
  /** Whether another changeset the page names has its name, and that of the one it reverts. */
  twin: boolean;
  revertsTwin: boolean;
  /** Lists the changesets of the account who made it alone, its name; none when the list is theirs already. */
  only: { name: string; choose: () => void } | undefined;
  left: () => void;
  linkRef?: Ref<HTMLAnchorElement>;
  /** The time the list was shown at, which its changesets' times are said before. */
  now: number;
}) {
  const t = useT();
  const { locale } = useStore().preferences;
  const row = useRef<HTMLLIElement>(null);
  useFocusLeaving(row, left);
  const name = changesetName(changeset, twin, t, locale);
  const newest = changeset.reverted_by[0];
  const reverts = changeset.reverts;
  const summary = isolate((named) => changesText(changeset, t, locale, named));
  return (
    <li ref={row} className={stackedLinesClass}>
      <p className={cn(lineTitleClass, "flex-wrap gap-x-3 gap-y-1")}>
        <Link ref={linkRef} to={href(changeset.id)} state={arrived} aria-label={name} className={lineLinkClass}>
          {byText(changeset, t)}
        </Link>
        <IngestMark count={changeset.ingest_count} />
        {/* -my-1: the 30px pill makes the 20px line no higher (review B's P5B-R1-N5). */}
        {only !== undefined && (
          <Button
            variant="ghost"
            size="sm"
            shape="pill"
            className={cn("-my-1", revealClass)}
            aria-label={only.name}
            onClick={only.choose}
          >
            {t("history.only")}
          </Button>
        )}
      </p>
      <p className={factsClass}>
        <Isolated {...isolate((named) => clientText(changeset.client, t, named))} />
        {" · "}
        <time dateTime={changeset.created_at} title={formatDateTime(changeset.created_at, locale)}>
          {formatRelative(changeset.created_at, now, locale)}
        </time>
        {twin && (
          <>
            {" · "}
            {t("history.id", { id: shortId(changeset.id) })}
          </>
        )}
      </p>
      {changeset.message !== null && (
        <p className="text-sm break-words">
          <bdi>{changeset.message}</bdi>
        </p>
      )}
      {summary.text !== "" && (
        <p className="text-sm break-words">
          <Isolated {...summary} />
        </p>
      )}
      <p className={factsClass}>{countsText(changeset, t, locale)}</p>
      <IngestSources changeset={changeset} limit={3} className={factsClass} />
      {(newest !== undefined || changeset.trimmed || changeset.kind === "revert") && (
        <p className="flex flex-wrap items-center gap-2 text-compact">
          {newest !== undefined && (
            <Link
              to={href(newest)}
              state={arrived}
              aria-label={t("history.revertedLabel", { name })}
              // A mark as Trimmed is, one shape of them in the row (decision 930): its edge and words darker as it is
              // hovered, as a card is, not underlined, which its border would cross (review B's P5B-R1-N3).
              className={cn(markClass, "hover:border-border-strong hover:text-foreground")}
            >
              {t("history.reverted")}
            </Link>
          )}
          {changeset.trimmed && <Mark>{t("history.trimmed")}</Mark>}
          {changeset.kind === "revert" &&
            (reverts === null ? (
              <span className="text-muted-foreground">{t("history.revertsPurged")}</span>
            ) : (
              <Link to={href(reverts.id)} state={arrived} className={lineLinkClass}>
                <Isolated
                  {...isolate((named) =>
                    t("history.reverts", { name: changesetName(reverts, revertsTwin, t, locale, named) })
                  )}
                />
              </Link>
            ))}
        </p>
      )}
    </li>
  );
});
